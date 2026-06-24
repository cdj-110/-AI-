import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import mqtt, { MqttClient } from 'mqtt';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../prisma/prisma.service';

interface ConfigReply {
  taskId?: string;
  status?: 'APPLIED' | 'FAILED';
  message?: string;
}

@Injectable()
export class RemoteConfigMqttService implements OnModuleInit, OnModuleDestroy {
  private client?: MqttClient;
  private readonly configWaiters = new Map<string, Array<(value: { config: unknown; reportedAt: Date }) => void>>();

  constructor(private readonly prisma: PrismaService) {}

  onModuleInit() {
    this.client = mqtt.connect(process.env.MQTT_URL ?? 'mqtt://localhost:1883', {
      clientId: `weikong-api-config-${Date.now()}`,
      username: process.env.MQTT_INGEST_USERNAME ?? process.env.MQTT_USERNAME ?? 'platform-ingest',
      password: process.env.MQTT_INGEST_PASSWORD ?? process.env.MQTT_PASSWORD ?? 'platform-ingest-secret',
      reconnectPeriod: 3000,
      connectTimeout: 5000,
    });
    this.client.on('connect', () => {
      console.log('[remote-config] mqtt connected');
      this.client?.subscribe([
        'weikong/gateways/+/config/reply',
        'weikong/gateways/+/config/reported',
      ], { qos: 1 }, (error, granted) => {
        if (error) console.error('[remote-config] subscribe failed', error.message);
        else console.log('[remote-config] subscribed', (granted ?? []).map((item) => item.topic).join(', '));
      });
    });
    this.client.on('message', (topic, payload) => {
      if (topic.endsWith('/config/reported')) void this.handleReported(topic, payload);
      else void this.handleReply(topic, payload);
    });
    this.client.on('error', (error) => console.error('[remote-config] mqtt error', error.message));
  }

  onModuleDestroy() {
    this.client?.end(true);
  }

  async publish(deviceKey: string, task: { id: string; version: number; config: unknown }) {
    if (!this.client?.connected) throw new Error('MQTT 未连接，远程配置任务暂时无法发送');
    const topic = `weikong/gateways/${deviceKey}/config/set`;
    const payload = JSON.stringify({ taskId: task.id, version: task.version, config: task.config, issuedAt: new Date().toISOString() });
    await new Promise<void>((resolve, reject) => {
      this.client!.publish(topic, payload, { qos: 1 }, (error) => error ? reject(error) : resolve());
    });
  }

  async requestConfig(deviceKey: string) {
    if (!this.client?.connected) throw new Error('MQTT 未连接，暂时无法读取网关配置');
    return new Promise<{ config: unknown; reportedAt: Date }>((resolve, reject) => {
      let timer: ReturnType<typeof setTimeout>;
      const wrappedResolve = (value: { config: unknown; reportedAt: Date }) => {
        clearTimeout(timer);
        resolve(value);
      };
      const waiters = this.configWaiters.get(deviceKey) ?? [];
      waiters.push(wrappedResolve);
      this.configWaiters.set(deviceKey, waiters);
      timer = setTimeout(() => {
        this.removeWaiter(deviceKey, wrappedResolve);
        reject(new Error('网关配置读取超时，请确认网关在线'));
      }, 5000);
      this.client!.publish(`weikong/gateways/${deviceKey}/config/get`, '{}', { qos: 1 }, (error) => {
        if (!error) return;
        clearTimeout(timer);
        this.removeWaiter(deviceKey, wrappedResolve);
        reject(error);
      });
    });
  }

  private async handleReported(topic: string, payload: Buffer) {
    const match = topic.match(/^weikong\/gateways\/([^/]+)\/config\/reported$/);
    if (!match) return;
    let body: { config?: unknown };
    try {
      body = JSON.parse(payload.toString()) as { config?: unknown };
    } catch {
      return;
    }
    if (!body.config || typeof body.config !== 'object' || Array.isArray(body.config)) return;
    const reportedAt = new Date();
    await this.prisma.device.updateMany({
      where: { deviceKey: match[1], deviceType: 'GATEWAY' },
      data: { reportedConfig: body.config as Prisma.InputJsonValue, reportedConfigAt: reportedAt },
    });
    const waiters = this.configWaiters.get(match[1]) ?? [];
    this.configWaiters.delete(match[1]);
    for (const resolve of waiters) resolve({ config: body.config, reportedAt });
  }

  private removeWaiter(deviceKey: string, waiter: (value: { config: unknown; reportedAt: Date }) => void) {
    const next = (this.configWaiters.get(deviceKey) ?? []).filter((item) => item !== waiter);
    if (next.length) this.configWaiters.set(deviceKey, next);
    else this.configWaiters.delete(deviceKey);
  }

  private async handleReply(topic: string, payload: Buffer) {
    const match = topic.match(/^weikong\/gateways\/([^/]+)\/config\/reply$/);
    if (!match) return;
    let reply: ConfigReply;
    try {
      reply = JSON.parse(payload.toString()) as ConfigReply;
    } catch {
      return;
    }
    if (!reply.taskId || !['APPLIED', 'FAILED'].includes(reply.status ?? '')) return;
    const task = await this.prisma.remoteConfigTask.findUnique({ where: { id: reply.taskId }, include: { device: true } });
    if (!task || task.device.deviceKey !== match[1]) return;
    await this.prisma.remoteConfigTask.update({
      where: { id: task.id },
      data: {
        status: reply.status,
        error: reply.status === 'FAILED' ? (reply.message || '网关应用配置失败') : null,
        completedAt: new Date(),
      },
    });
  }
}

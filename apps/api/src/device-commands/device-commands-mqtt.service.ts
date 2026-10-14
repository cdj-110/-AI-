import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { DeviceCommand } from '@prisma/client';
import mqtt, { MqttClient } from 'mqtt';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class DeviceCommandsMqttService implements OnModuleInit, OnModuleDestroy {
  private client?: MqttClient;
  private timer?: ReturnType<typeof setInterval>;
  private ready = false;
  private retry?: ReturnType<typeof setTimeout>;
  constructor(private readonly prisma: PrismaService) {}

  onModuleInit() {
    this.client = mqtt.connect(process.env.MQTT_URL ?? 'mqtt://localhost:1883', { clientId: `weikong-api-commands-${Date.now()}`, username: process.env.MQTT_INGEST_USERNAME ?? 'platform-ingest', password: process.env.MQTT_INGEST_PASSWORD ?? 'platform-ingest-secret', reconnectPeriod: 3000, connectTimeout: 5000 });
    this.client.on('connect', () => this.subscribe());
    this.client.on('close', () => { this.ready = false; });
    this.client.on('error', error => console.error('[device-commands] MQTT', error.message));
    this.client.on('message', (topic, payload, packet) => { if (!packet.retain) void this.handleReply(topic, payload).catch(error => console.error('[device-commands] reply', error.message)); });
    this.timer = setInterval(() => { void this.expire().catch(error => console.error('[device-commands] expiry', error.message)); }, 5000);
  }
  onModuleDestroy() { if (this.timer) clearInterval(this.timer); if (this.retry) clearTimeout(this.retry); this.client?.end(true); }

  private subscribe() {
    if (this.retry) clearTimeout(this.retry);
    if (!this.client?.connected) return;
    this.client.subscribe('weikong/gateways/+/commands/reply', { qos: 1 }, (error, granted) => {
      this.ready = !error && granted?.length === 1 && granted[0].qos !== 128;
      if (!this.ready) this.retry = setTimeout(() => this.subscribe(), 3000);
    });
  }

  async expire() {
    await this.prisma.deviceCommand.updateMany({ where: { status: 'SENT', expiresAt: { lte: new Date() } }, data: { status: 'TIMEOUT', message: '30 秒内未收到网关回执，结果未知；请检查实际值，不要盲目重试', completedAt: new Date() } });
  }

  async publish(gatewayKey: string, task: DeviceCommand) {
    if (!this.client?.connected || !this.ready) throw Object.assign(new Error('下发通道未就绪'), { neverSent: true });
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('发送确认超时')), 5000);
      this.client!.publish(`weikong/gateways/${gatewayKey}/commands/set`, JSON.stringify({ commandId: task.id, deviceKey: task.deviceKey, metric: task.metric, value: task.value, issuedAt: task.createdAt.toISOString(), expiresAt: task.expiresAt.toISOString() }), { qos: 1, retain: false }, error => { clearTimeout(timer); error ? reject(error) : resolve(); });
    });
  }

  async handleReply(topic: string, payload: Buffer) {
    const match = /^weikong\/gateways\/([^/]+)\/commands\/reply$/.exec(topic);
    if (!match || payload.length > 8192) return;
    let reply: { commandId?: string; deviceKey?: string; metric?: string; status?: string; message?: string };
    try { reply = JSON.parse(payload.toString()); } catch { return; }
    if (!reply || typeof reply !== 'object' || typeof reply.commandId !== 'string' || !['SUCCEEDED', 'FAILED', 'UNKNOWN'].includes(reply.status ?? '')) return;
    const task = await this.prisma.deviceCommand.findUnique({ where: { id: reply.commandId }, include: { gateway: true, device: true } });
    if (!task || task.gateway.deviceKey !== match[1] || task.deviceKey !== reply.deviceKey || task.device.deviceKey !== task.deviceKey || task.metric !== reply.metric || task.device.gatewayId !== task.gatewayId || task.device.tenantId !== task.gateway.tenantId) return;
    // A valid late receipt can resolve an ambiguous timeout; a final result is immutable.
    await this.prisma.deviceCommand.updateMany({ where: { id: task.id, status: { in: ['SENT', 'TIMEOUT'] } }, data: { status: reply.status === 'UNKNOWN' ? 'TIMEOUT' : reply.status, message: typeof reply.message === 'string' ? reply.message.slice(0, 1000) : reply.status === 'SUCCEEDED' ? '网关已执行点位写入' : '网关写入失败', completedAt: new Date() } });
  }
}

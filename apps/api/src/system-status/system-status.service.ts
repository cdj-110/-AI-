import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { createConnection } from 'node:net';
import { Pool } from 'pg';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

export type HealthState = 'UP' | 'DOWN' | 'WARN';

export interface HealthItem {
  name: string;
  state: HealthState;
  latencyMs?: number;
  message: string;
}

@Injectable()
export class SystemStatusService implements OnModuleDestroy {
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@127.0.0.1:5433/weikong_ts',
  });

  constructor(private readonly prisma: PrismaService) {}

  async onModuleDestroy() {
    await this.timescale.end();
  }

  async status(actor: AuthUser) {
    const [postgres, timescale, emqx, recentTelemetry, recentDeviceLog] = await Promise.all([
      this.checkPostgres(),
      this.checkTimescale(),
      this.checkTcp('EMQX MQTT', process.env.MQTT_PUBLIC_HOST ?? '127.0.0.1', Number(process.env.MQTT_PUBLIC_PORT ?? 1883)),
      this.safeRecentTelemetry(actor),
      this.safeRecentDeviceLog(actor),
    ]);
    const services = [
      { name: 'API', state: 'UP' as HealthState, message: '接口服务运行中' },
      postgres,
      timescale,
      emqx,
    ];
    const overall: HealthState = services.some((item) => item.state === 'DOWN') ? 'DOWN' : services.some((item) => item.state === 'WARN') ? 'WARN' : 'UP';
    return {
      overall,
      checkedAt: new Date(),
      services,
      recentTelemetry,
      recentDeviceLog,
    };
  }

  private async safeRecentTelemetry(actor: AuthUser) {
    try {
      return await this.recentTelemetry(actor);
    } catch {
      return null;
    }
  }

  private async safeRecentDeviceLog(actor: AuthUser) {
    try {
      return await this.recentDeviceLog(actor);
    } catch {
      return null;
    }
  }

  private async checkPostgres(): Promise<HealthItem> {
    const startedAt = Date.now();
    try {
      await this.prisma.$queryRaw`SELECT 1`;
      return { name: 'PostgreSQL', state: 'UP', latencyMs: Date.now() - startedAt, message: '主业务数据库连接正常' };
    } catch (error) {
      return { name: 'PostgreSQL', state: 'DOWN', latencyMs: Date.now() - startedAt, message: this.errorMessage(error) };
    }
  }

  private async checkTimescale(): Promise<HealthItem> {
    const startedAt = Date.now();
    try {
      await this.timescale.query('SELECT 1');
      return { name: 'TimescaleDB', state: 'UP', latencyMs: Date.now() - startedAt, message: '遥测时序库连接正常' };
    } catch (error) {
      return { name: 'TimescaleDB', state: 'DOWN', latencyMs: Date.now() - startedAt, message: this.errorMessage(error) };
    }
  }

  private checkTcp(name: string, host: string, port: number): Promise<HealthItem> {
    const startedAt = Date.now();
    return new Promise((resolve) => {
      const socket = createConnection({ host, port, timeout: 1800 });
      const finish = (item: HealthItem) => {
        socket.destroy();
        resolve(item);
      };

      socket.once('connect', () => {
        finish({ name, state: 'UP', latencyMs: Date.now() - startedAt, message: `${host}:${port} 可连接` });
      });
      socket.once('timeout', () => {
        finish({ name, state: 'DOWN', latencyMs: Date.now() - startedAt, message: `${host}:${port} 连接超时` });
      });
      socket.once('error', (error) => {
        finish({ name, state: 'DOWN', latencyMs: Date.now() - startedAt, message: error.message });
      });
    });
  }

  private async recentTelemetry(actor: AuthUser) {
    const devices = await this.prisma.device.findMany({
      where: actor.role === 'SUPER_ADMIN' ? {} : { tenantId: actor.tenantId ?? undefined },
      select: { deviceKey: true, name: true },
    });
    if (!devices.length) return null;
    const keys = devices.map((device) => device.deviceKey);
    const result = await this.timescale.query<{ time: Date; device_key: string }>(
      'SELECT time, device_key FROM telemetry_events WHERE device_key = ANY($1) ORDER BY time DESC LIMIT 1',
      [keys],
    );
    const latest = result.rows[0];
    if (!latest) return null;
    return {
      time: latest.time,
      deviceKey: latest.device_key,
      deviceName: devices.find((device) => device.deviceKey === latest.device_key)?.name,
    };
  }

  private async recentDeviceLog(actor: AuthUser) {
    return this.prisma.deviceLog.findFirst({
      where: actor.role === 'SUPER_ADMIN' ? {} : { tenantId: actor.tenantId ?? undefined },
      orderBy: { createdAt: 'desc' },
      select: { createdAt: true, deviceKey: true, deviceName: true, type: true, message: true },
    });
  }

  private errorMessage(error: unknown) {
    return error instanceof Error ? error.message : String(error);
  }
}

import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { Pool } from 'pg';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class DashboardService implements OnModuleDestroy {
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5433/weikong_ts',
  });

  constructor(private readonly prisma: PrismaService) {}

  async summary(user: AuthUser) {
    const deviceWhere: Prisma.DeviceWhereInput = user.role === 'SUPER_ADMIN' ? {} : { tenantId: user.tenantId ?? undefined };
    const alarmWhere: Prisma.AlarmWhereInput = {
      ...(user.role === 'SUPER_ADMIN' ? {} : { tenantId: user.tenantId ?? undefined }),
      status: 'OPEN',
    };
    const [tenantCount, deviceCount, onlineDeviceCount, offlineDeviceCount, alarmCount] = await this.prisma.$transaction([
      user.role === 'SUPER_ADMIN' ? this.prisma.tenant.count() : this.prisma.tenant.count({ where: { id: user.tenantId ?? undefined } }),
      this.prisma.device.count({ where: deviceWhere }),
      this.prisma.device.count({ where: { ...deviceWhere, status: 'ONLINE' } }),
      this.prisma.device.count({ where: { ...deviceWhere, status: 'OFFLINE' } }),
      this.prisma.alarm.count({ where: alarmWhere }),
    ]);
    return {
      tenantCount,
      deviceCount,
      onlineDeviceCount,
      offlineDeviceCount,
      alarmCount,
    };
  }

  async trends(user: AuthUser) {
    const devices = await this.prisma.device.findMany({
      where: user.role === 'SUPER_ADMIN' ? {} : { tenantId: user.tenantId ?? undefined },
      select: { deviceKey: true },
    });
    const deviceKeys = devices.map((device) => device.deviceKey);
    if (!deviceKeys.length) return { temperature: [] };
    const result = await this.timescale.query<{ time: Date; device_key: string; temperature: number | null }>(
      `SELECT time, device_key, (metrics->>'temperature')::double precision AS temperature
       FROM telemetry_events
       WHERE device_key = ANY($1) AND metrics ? 'temperature'
       ORDER BY time DESC
       LIMIT 30`,
      [deviceKeys],
    );
    return {
      temperature: result.rows.reverse().map((row) => ({
        time: row.time,
        deviceKey: row.device_key,
        value: row.temperature,
      })),
    };
  }

  async onModuleDestroy() {
    await this.timescale.end();
  }
}

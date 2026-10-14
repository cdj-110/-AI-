import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { Pool } from 'pg';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { deviceAccessWhere } from '../common/user-permissions';
import { SubscriptionLimitsService } from '../subscriptions/subscription-limits.service';

@Injectable()
export class DashboardService implements OnModuleDestroy {
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong@localhost:5433/weikong_ts',
  });

  constructor(private readonly prisma: PrismaService, private readonly limits: SubscriptionLimitsService) {}

  async summary(user: AuthUser) {
    const deviceWhere: Prisma.DeviceWhereInput = deviceAccessWhere(user);
    const alarmWhere: Prisma.AlarmWhereInput = {
      ...(user.role === 'SUPER_ADMIN' ? {} : { tenantId: user.tenantId ?? undefined }),
      ...(user.role === 'TENANT_USER' && user.deviceScopeMode === 'CUSTOM' ? { device: deviceWhere } : {}),
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
      where: deviceAccessWhere(user),
      select: { id: true, deviceKey: true, tenantId: true },
    });
    const deviceIds = devices.map((device) => device.id);
    if (!deviceIds.length) return { temperature: [] };
    const deviceKeyById = new Map(devices.map((device) => [device.id, device.deviceKey]));
    const windows = new Map(await Promise.all([...new Set(devices.map(d => d.tenantId))].map(async id => [id, await this.limits.history(id)] as const)));
    const retention = JSON.stringify(devices.map(d => ({ id: d.id, earliestAt: windows.get(d.tenantId)!.queryEarliestAt.toISOString() })));
    const result = await this.timescale.query<{ time: Date; device_id: string; temperature: number | null }>(
      `SELECT time, device_id, (metrics->>'temperature')::double precision AS temperature
       FROM telemetry_events
       WHERE device_id = ANY($1) AND metrics ? 'temperature'
       AND time <= NOW() AND EXISTS (SELECT 1 FROM jsonb_to_recordset($2::jsonb) AS r(id text, "earliestAt" timestamptz) WHERE r.id = device_id AND time >= r."earliestAt")
       ORDER BY time DESC
       LIMIT 30`,
      [deviceIds, retention],
    );
    return {
      temperature: result.rows.reverse().map((row) => ({
        time: row.time,
        deviceKey: deviceKeyById.get(row.device_id),
        value: row.temperature,
      })),
    };
  }

  async onModuleDestroy() {
    await this.timescale.end();
  }
}

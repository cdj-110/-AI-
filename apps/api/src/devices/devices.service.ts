import { BadRequestException, ForbiddenException, Injectable, MessageEvent, NotFoundException, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { Pool, PoolClient } from 'pg';
import { filter, map, Observable, Subject } from 'rxjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateDeviceDto } from './dto/create-device.dto';
import { CreateDeviceAlarmRuleDto } from './dto/create-device-alarm-rule.dto';
import { ImportDeviceMetricsDto } from './dto/import-device-metrics.dto';
import { ReportDeviceStatusDto } from './dto/report-device-status.dto';
import { UpdateDeviceMetricDto } from './dto/update-device-metric.dto';
import { UpdateDeviceDto } from './dto/update-device.dto';

@Injectable()
export class DevicesService implements OnModuleDestroy {
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5433/weikong_ts',
  });
  private readonly postgres = new Pool({
    connectionString: (process.env.DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5432/weikong_iot?schema=public').replace('?schema=public', ''),
  });
  private readonly telemetryUpdates = new Subject<{ deviceKey: string; time: string; metrics: Record<string, unknown> }>();
  private readonly statusUpdates = new Subject<{ id: string; tenantId: string; deviceKey: string; name: string; status: string; lastSeenAt?: Date | null }>();
  private telemetryListener?: PoolClient;
  private statusListener?: PoolClient;
  private destroying = false;

  constructor(private readonly prisma: PrismaService) {
    void this.listenForTelemetry();
    void this.listenForDeviceStatus();
  }

  async findAll(actor: AuthUser, page: number, pageSize: number, keyword: string) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.DeviceWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(keyword
        ? { OR: [{ name: { contains: keyword, mode: 'insensitive' } }, { deviceKey: { contains: keyword, mode: 'insensitive' } }] }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.device.findMany({
        where,
        include: { tenant: { select: { id: true, name: true } } },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.device.count({ where }),
    ]);
    return { items, total, page: safePage, pageSize: safePageSize };
  }

  async create(actor: AuthUser, dto: CreateDeviceDto) {
    const tenantId = actor.role === 'SUPER_ADMIN' ? await this.getDefaultTenantId() : actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('设备必须指定所属租户');
    const exists = await this.prisma.device.findUnique({ where: { deviceKey: dto.deviceKey } });
    if (exists) throw new BadRequestException('设备编号已存在');
    const { templateDeviceId, ...deviceData } = dto;
    const templateMetrics = templateDeviceId
      ? await this.prisma.deviceMetric.findMany({ where: { deviceId: (await this.getAccessibleDevice(actor, templateDeviceId)).id } })
      : [];
    return this.prisma.$transaction(async (tx) => {
      const device = await tx.device.create({ data: { ...deviceData, tenantId, status: 'OFFLINE' } });
      if (templateMetrics.length) {
        await tx.deviceMetric.createMany({
          data: templateMetrics.map(({ identifier, name, dataType, unit, decimals, accessMode, enabled, sortOrder }) => ({
            deviceId: device.id,
            identifier,
            name,
            dataType,
            unit,
            decimals,
            accessMode,
            enabled,
            sortOrder,
          })),
        });
      }
      return tx.device.findUnique({ where: { id: device.id }, include: { tenant: { select: { id: true, name: true } } } });
    });
  }

  async modelTemplates(actor: AuthUser) {
    return this.prisma.device.findMany({
      where: {
        ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
        metrics: { some: {} },
      },
      select: { id: true, name: true, deviceKey: true, _count: { select: { metrics: true } } },
      orderBy: { createdAt: 'desc' },
      take: 100,
    });
  }

  async findOne(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    return device;
  }

  async update(actor: AuthUser, id: string, dto: UpdateDeviceDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const { templateDeviceId: _templateDeviceId, ...deviceData } = dto;
    return this.prisma.device.update({
      where: { id: device.id },
      data: deviceData,
      include: { tenant: { select: { id: true, name: true } } },
    });
  }

  async remove(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    await this.prisma.device.delete({ where: { id: device.id } });
    return { message: '删除成功' };
  }

  async reportStatus(actor: AuthUser, id: string, dto: ReportDeviceStatusDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const updated = await this.prisma.device.update({
      where: { id: device.id },
      data: {
        status: dto.status,
        ...(dto.status === 'ONLINE' ? { lastSeenAt: new Date() } : {}),
      },
      include: { tenant: { select: { id: true, name: true } } },
    });
    if (device.status !== updated.status) this.statusUpdates.next(updated);
    return updated;
  }

  async telemetry(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const result = await this.timescale.query<{ time: Date; metrics: Record<string, unknown> }>(
      'SELECT time, metrics FROM telemetry_events WHERE device_key = $1 ORDER BY time DESC LIMIT 20',
      [device.deviceKey],
    );
    return { deviceKey: device.deviceKey, items: result.rows };
  }

  async telemetryStream(actor: AuthUser, id: string): Promise<Observable<MessageEvent>> {
    const device = await this.getAccessibleDevice(actor, id);
    return this.telemetryUpdates.pipe(
      filter((event) => event.deviceKey === device.deviceKey),
      map((event) => ({ data: event })),
    );
  }

  statusStream(actor: AuthUser): Observable<MessageEvent> {
    return this.statusUpdates.pipe(
      filter((device) => actor.role === 'SUPER_ADMIN' || device.tenantId === actor.tenantId),
      map((device) => ({ data: device })),
    );
  }

  async metrics(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    return this.prisma.deviceMetric.findMany({ where: { deviceId: device.id }, orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] });
  }

  async updateMetric(actor: AuthUser, id: string, metricId: string, dto: UpdateDeviceMetricDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const metric = await this.prisma.deviceMetric.findFirst({ where: { id: metricId, deviceId: device.id } });
    if (!metric) throw new NotFoundException('指标不存在');
    return this.prisma.deviceMetric.update({ where: { id: metric.id }, data: dto });
  }

  async importMetrics(actor: AuthUser, id: string, dto: ImportDeviceMetricsDto) {
    const target = await this.getAccessibleDevice(actor, id);
    const source = await this.getAccessibleDevice(actor, dto.templateDeviceId);
    if (source.id === target.id) throw new BadRequestException('不能从当前设备导入物模型');
    const sourceMetrics = await this.prisma.deviceMetric.findMany({ where: { deviceId: source.id } });
    if (!sourceMetrics.length) throw new BadRequestException('模板设备暂无可导入的物模型');
    const existingMetrics = await this.prisma.deviceMetric.findMany({ where: { deviceId: target.id }, select: { identifier: true } });
    const existingIdentifiers = new Set(existingMetrics.map((metric) => metric.identifier));
    let created = 0;
    let updated = 0;
    await this.prisma.$transaction(async (tx) => {
      for (const metric of sourceMetrics) {
        const data = {
          name: metric.name,
          dataType: metric.dataType,
          unit: metric.unit,
          decimals: metric.decimals,
          accessMode: metric.accessMode,
          enabled: metric.enabled,
          sortOrder: metric.sortOrder,
        };
        if (existingIdentifiers.has(metric.identifier)) {
          if (!dto.overwrite) continue;
          await tx.deviceMetric.update({
            where: { deviceId_identifier: { deviceId: target.id, identifier: metric.identifier } },
            data,
          });
          updated += 1;
          continue;
        }
        await tx.deviceMetric.create({ data: { deviceId: target.id, identifier: metric.identifier, ...data } });
        created += 1;
      }
    });
    return { created, updated, skipped: sourceMetrics.length - created - updated };
  }

  async alarmRules(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    return this.prisma.deviceAlarmRule.findMany({ where: { deviceId: device.id }, orderBy: { createdAt: 'desc' } });
  }

  async createAlarmRule(actor: AuthUser, id: string, dto: CreateDeviceAlarmRuleDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const metric = await this.prisma.deviceMetric.findUnique({ where: { deviceId_identifier: { deviceId: device.id, identifier: dto.identifier } } });
    if (!metric) throw new BadRequestException('请先等待设备上报该指标');
    return this.prisma.deviceAlarmRule.upsert({
      where: { deviceId_identifier_operator: { deviceId: device.id, identifier: dto.identifier, operator: dto.operator } },
      create: { deviceId: device.id, ...dto },
      update: dto,
    });
  }

  async removeAlarmRule(actor: AuthUser, id: string, ruleId: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const rule = await this.prisma.deviceAlarmRule.findFirst({ where: { id: ruleId, deviceId: device.id } });
    if (!rule) throw new NotFoundException('告警规则不存在');
    await this.prisma.deviceAlarmRule.delete({ where: { id: rule.id } });
    return { message: '删除成功' };
  }

  async onModuleDestroy() {
    this.destroying = true;
    this.telemetryListener?.release();
    this.statusListener?.release();
    await this.timescale.end();
    await this.postgres.end();
  }

  private async listenForTelemetry() {
    try {
      this.telemetryListener = await this.timescale.connect();
      await this.telemetryListener.query('LISTEN telemetry_updates');
      this.telemetryListener.on('notification', (notification) => {
        if (!notification.payload) return;
        try {
          this.telemetryUpdates.next(JSON.parse(notification.payload));
        } catch {
          console.warn('[api] ignored invalid telemetry notification');
        }
      });
      this.telemetryListener.on('error', (error) => {
        console.error('[api] telemetry listener error', error);
        this.telemetryListener?.release();
        this.telemetryListener = undefined;
        if (!this.destroying) setTimeout(() => void this.listenForTelemetry(), 3000);
      });
    } catch (error) {
      console.error('[api] failed to listen for telemetry updates', error);
      if (!this.destroying) setTimeout(() => void this.listenForTelemetry(), 3000);
    }
  }

  private async listenForDeviceStatus() {
    try {
      this.statusListener = await this.postgres.connect();
      await this.statusListener.query('LISTEN device_status_updates');
      this.statusListener.on('notification', (notification) => {
        if (!notification.payload) return;
        void this.publishDeviceStatus(notification.payload);
      });
      this.statusListener.on('error', (error) => {
        console.error('[api] device status listener error', error);
        this.statusListener?.release();
        this.statusListener = undefined;
        if (!this.destroying) setTimeout(() => void this.listenForDeviceStatus(), 3000);
      });
    } catch (error) {
      console.error('[api] failed to listen for device status updates', error);
      if (!this.destroying) setTimeout(() => void this.listenForDeviceStatus(), 3000);
    }
  }

  private async publishDeviceStatus(payload: string) {
    try {
      const parsed = JSON.parse(payload) as { id?: string };
      if (!parsed.id) return;
      const device = await this.prisma.device.findUnique({
        where: { id: parsed.id },
        select: { id: true, tenantId: true, deviceKey: true, name: true, status: true, lastSeenAt: true },
      });
      if (device) this.statusUpdates.next(device);
    } catch (error) {
      console.warn('[api] ignored invalid device status notification', error);
    }
  }

  private async getDefaultTenantId() {
    const tenant = await this.prisma.tenant.findFirst({ where: { name: '系统默认租户' } });
    if (tenant) return tenant.id;
    return (await this.prisma.tenant.create({ data: { name: '系统默认租户' } })).id;
  }

  private async getAccessibleDevice(actor: AuthUser, id: string) {
    const device = await this.prisma.device.findUnique({ where: { id }, include: { tenant: { select: { id: true, name: true } } } });
    if (!device) throw new NotFoundException('设备不存在');
    if (actor.role !== 'SUPER_ADMIN' && device.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该设备');
    return device;
  }
}

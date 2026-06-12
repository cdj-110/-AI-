import { BadRequestException, ForbiddenException, Injectable, MessageEvent, NotFoundException, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { randomBytes } from 'crypto';
import * as bcrypt from 'bcryptjs';
import { Pool, PoolClient } from 'pg';
import { filter, map, Observable, Subject } from 'rxjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateDeviceDto } from './dto/create-device.dto';
import { CreateDeviceAlarmRuleDto } from './dto/create-device-alarm-rule.dto';
import { ImportDeviceMetricsDto } from './dto/import-device-metrics.dto';
import { ReportDeviceStatusDto } from './dto/report-device-status.dto';
import { UpdateDeviceAlarmRuleDto } from './dto/update-device-alarm-rule.dto';
import { UpdateDeviceMetricDto } from './dto/update-device-metric.dto';
import { UpdateDeviceDto } from './dto/update-device.dto';

@Injectable()
export class DevicesService implements OnModuleDestroy {
  // TimescaleDB 专门存储遥测时序数据，避免把高频上报压到业务库里。
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5433/weikong_ts',
  });
  // 业务库连接用于 LISTEN/NOTIFY 监听设备状态变化，Prisma 本身不提供持久监听能力。
  private readonly postgres = new Pool({
    connectionString: (process.env.DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5432/weikong_iot?schema=public').replace('?schema=public', ''),
  });
  // API 内存事件总线：数据库通知进来后再转成 SSE 推给浏览器。
  private readonly telemetryUpdates = new Subject<{ deviceKey: string; time: string; metrics: Record<string, unknown> }>();
  private readonly statusUpdates = new Subject<{ id: string; tenantId: string; deviceKey: string; name: string; status: string; lastSeenAt?: Date | null }>();
  private telemetryListener?: PoolClient;
  private statusListener?: PoolClient;
  private destroying = false;

  constructor(private readonly prisma: PrismaService) {
    void this.listenForTelemetry();
    void this.listenForDeviceStatus();
  }

  async findAll(actor: AuthUser, page: number, pageSize: number, keyword: string, deviceType?: string) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const safeDeviceType = ['GATEWAY', 'GATEWAY_CHILD', 'DIRECT'].includes(deviceType ?? '') ? deviceType : undefined;
    const where: Prisma.DeviceWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(safeDeviceType ? { deviceType: safeDeviceType } : {}),
      ...(keyword
        ? { OR: [{ name: { contains: keyword, mode: 'insensitive' } }, { deviceKey: { contains: keyword, mode: 'insensitive' } }] }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.device.findMany({
        where,
        include: {
          tenant: { select: { id: true, name: true } },
          gateway: { select: { id: true, name: true, deviceKey: true, status: true, lastSeenAt: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.device.count({ where }),
    ]);
    return { items: items.map((item) => this.toPublicDevice(item)), total, page: safePage, pageSize: safePageSize };
  }

  async create(actor: AuthUser, dto: CreateDeviceDto) {
    const tenantId = actor.role === 'SUPER_ADMIN' ? await this.getDefaultTenantId() : actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('设备必须指定所属租户');
    const exists = await this.prisma.device.findUnique({ where: { deviceKey: dto.deviceKey } });
    if (exists) throw new BadRequestException('设备编号已存在');
    const { templateDeviceId, modelTemplateId, ...deviceData } = dto;
    await this.validateGatewayRelation(actor, deviceData.deviceType ?? 'DIRECT', deviceData.gatewayId);
    const credentials = await this.generateMqttCredentials(dto.deviceKey);
    // 物模型模板只在创建设备时复制一份，后续设备字段可以独立调整。
    const templateMetrics = await this.resolveTemplateMetrics(actor, { templateDeviceId, modelTemplateId });
    return this.prisma.$transaction(async (tx) => {
      const device = await tx.device.create({
        data: {
          ...deviceData,
          tenantId,
          status: 'OFFLINE',
          mqttClientId: credentials.clientId,
          mqttUsername: credentials.username,
          mqttPasswordHash: credentials.passwordHash,
          mqttPasswordUpdatedAt: new Date(),
        },
      });
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
      const created = await tx.device.findUnique({
        where: { id: device.id },
        include: {
          tenant: { select: { id: true, name: true } },
          gateway: { select: { id: true, name: true, deviceKey: true, status: true, lastSeenAt: true } },
        },
      });
      return created ? { ...this.toPublicDevice(created), mqtt: this.mqttConnectionInfo(created, credentials.password) } : null;
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

  async gateways(actor: AuthUser) {
    return this.prisma.device.findMany({
      where: {
        ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
        deviceType: 'GATEWAY',
      },
      select: { id: true, name: true, deviceKey: true, status: true, lastSeenAt: true },
      orderBy: { createdAt: 'desc' },
      take: 200,
    });
  }

  async findOne(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    return { ...this.toPublicDevice(device), mqtt: this.mqttConnectionInfo(device) };
  }

  async update(actor: AuthUser, id: string, dto: UpdateDeviceDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const { templateDeviceId: _templateDeviceId, modelTemplateId: _modelTemplateId, ...deviceData } = dto;
    const nextDeviceType = deviceData.deviceType ?? device.deviceType;
    await this.validateGatewayRelation(actor, nextDeviceType, deviceData.gatewayId ?? device.gatewayId, device.id);
    await this.ensureGatewayCanChangeType(device.id, device.deviceType, nextDeviceType);
    const updateData: Prisma.DeviceUncheckedUpdateInput = {
      ...deviceData,
      ...(nextDeviceType !== 'GATEWAY_CHILD' ? { gatewayId: null } : {}),
    };
    const updated = await this.prisma.device.update({
      where: { id: device.id },
      data: updateData,
      include: {
        tenant: { select: { id: true, name: true } },
        gateway: { select: { id: true, name: true, deviceKey: true, status: true, lastSeenAt: true } },
      },
    });
    return this.toPublicDevice(updated);
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
    if (device.status !== updated.status) {
      await this.prisma.deviceLog.create({
        data: {
          tenantId: updated.tenantId,
          deviceId: updated.id,
          deviceKey: updated.deviceKey,
          deviceName: updated.name,
          type: updated.status,
          level: 'INFO',
          source: 'API_REPORT',
          message: updated.status === 'ONLINE' ? '设备已上线' : '设备已离线',
          detail: { operator: actor.username } as Prisma.InputJsonValue,
        },
      });
      this.statusUpdates.next(updated);
    }
    return this.toPublicDevice(updated);
  }

  async credentials(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    return this.mqttConnectionInfo(device);
  }

  async rotateCredentials(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const password = this.generateMqttPassword();
    const passwordHash = await bcrypt.hash(password, 10);
    const updated = await this.prisma.device.update({
      where: { id: device.id },
      data: { mqttPasswordHash: passwordHash, mqttPasswordUpdatedAt: new Date() },
      include: { tenant: { select: { id: true, name: true } } },
    });
    return this.mqttConnectionInfo(updated, password);
  }

  async telemetry(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceKeys = this.telemetryDeviceKeys(device);
    const result = await this.timescale.query<{ time: Date; deviceKey: string; metrics: Record<string, unknown> }>(
      'SELECT time, device_key as "deviceKey", metrics FROM telemetry_events WHERE device_key = ANY($1) ORDER BY time DESC LIMIT 20',
      [deviceKeys],
    );
    return { deviceKey: device.deviceKey, items: result.rows };
  }

  async telemetryTrend(actor: AuthUser, id: string, metric: string, range: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceKeys = this.telemetryDeviceKeys(device);
    const metricKey = metric?.trim();
    if (!metricKey) throw new BadRequestException('请选择趋势指标');
    // JSON 字段名会作为查询参数传给 TimescaleDB，先限制字符集，避免不受控 SQL/JSON key。
    if (!/^[\w.\-:]+$/.test(metricKey)) throw new BadRequestException('指标标识符格式不支持');
    const option = this.trendRangeOption(range);
    const result = await this.timescale.query<{ time: Date; value: number }>(
      `
        SELECT time_bucket($1::interval, time) AS time,
               AVG((metrics ->> $2)::double precision) AS value
        FROM telemetry_events
        WHERE device_key = ANY($3)
          AND time >= NOW() - $4::interval
          AND metrics ? $2
          AND jsonb_typeof(metrics -> $2) = 'number'
        GROUP BY 1
        ORDER BY 1 ASC
      `,
      [option.bucket, metricKey, deviceKeys, option.interval],
    );
    return {
      deviceKey: device.deviceKey,
      metric: metricKey,
      range: option.key,
      bucket: option.bucket,
      items: result.rows.map((row) => ({ time: row.time, value: Number(row.value) })),
    };
  }

  async telemetryStream(actor: AuthUser, id: string): Promise<Observable<MessageEvent>> {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceKeys = new Set(this.telemetryDeviceKeys(device));
    return this.telemetryUpdates.pipe(
      filter((event) => deviceKeys.has(event.deviceKey)),
      map((event) => ({ data: event })),
    );
  }

  statusStream(actor: AuthUser): Observable<MessageEvent> {
    return new Observable<MessageEvent>((subscriber) => {
      let liveSubscription: { unsubscribe: () => void } | undefined;
      // SSE 建连后先推一次快照，避免页面刷新后要等下一次设备状态变化才有数据。
      void this.statusSnapshot(actor)
        .then((devices) => {
          for (const device of devices) subscriber.next({ data: { ...device, snapshot: true } });
          liveSubscription = this.statusUpdates.pipe(
            filter((device) => actor.role === 'SUPER_ADMIN' || device.tenantId === actor.tenantId),
            map((device) => ({ data: device })),
          ).subscribe(subscriber);
        })
        .catch((error) => subscriber.error(error));
      return () => liveSubscription?.unsubscribe();
    });
  }

  async metrics(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceIds = this.metricDeviceIds(device);
    return this.prisma.deviceMetric.findMany({
      where: { deviceId: { in: deviceIds } },
      orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }],
    });
  }

  async updateMetric(actor: AuthUser, id: string, metricId: string, dto: UpdateDeviceMetricDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const metric = await this.prisma.deviceMetric.findFirst({ where: { id: metricId, deviceId: { in: this.metricDeviceIds(device) } } });
    if (!metric) throw new NotFoundException('指标不存在');
    return this.prisma.deviceMetric.update({ where: { id: metric.id }, data: dto });
  }

  async importMetrics(actor: AuthUser, id: string, dto: ImportDeviceMetricsDto) {
    const target = await this.getAccessibleDevice(actor, id);
    const sourceMetrics = await this.resolveImportMetrics(actor, target.id, dto);
    if (!sourceMetrics.length) throw new BadRequestException('暂无可导入的物模型字段');
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

  async updateAlarmRule(actor: AuthUser, id: string, ruleId: string, dto: UpdateDeviceAlarmRuleDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const rule = await this.prisma.deviceAlarmRule.findFirst({ where: { id: ruleId, deviceId: device.id } });
    if (!rule) throw new NotFoundException('告警规则不存在');
    if (dto.identifier) {
      const metric = await this.prisma.deviceMetric.findUnique({ where: { deviceId_identifier: { deviceId: device.id, identifier: dto.identifier } } });
      if (!metric) throw new BadRequestException('请先等待设备上报该指标');
    }
    return this.prisma.deviceAlarmRule.update({ where: { id: rule.id }, data: dto });
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

  private async statusSnapshot(actor: AuthUser) {
    return this.prisma.device.findMany({
      where: {
        ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      },
      select: { id: true, tenantId: true, deviceKey: true, name: true, status: true, lastSeenAt: true },
      orderBy: { createdAt: 'desc' },
      take: 500,
    });
  }

  private trendRangeOption(range: string) {
    // 范围越长 bucket 越大，控制前端点数，避免一天/一周趋势图过密。
    const options: Record<string, { key: string; interval: string; bucket: string }> = {
      '1m': { key: '1m', interval: '1 minute', bucket: '2 seconds' },
      '15m': { key: '15m', interval: '15 minutes', bucket: '15 seconds' },
      '30m': { key: '30m', interval: '30 minutes', bucket: '30 seconds' },
      '1h': { key: '1h', interval: '1 hour', bucket: '1 minute' },
      '3h': { key: '3h', interval: '3 hours', bucket: '3 minutes' },
      '6h': { key: '6h', interval: '6 hours', bucket: '5 minutes' },
      '1d': { key: '1d', interval: '1 day', bucket: '10 minutes' },
      '1w': { key: '1w', interval: '7 days', bucket: '1 hour' },
    };
    return options[range] ?? options['15m'];
  }

  private async getDefaultTenantId() {
    const tenant = await this.prisma.tenant.findFirst({ where: { name: '系统默认租户' } });
    if (tenant) return tenant.id;
    return (await this.prisma.tenant.create({ data: { name: '系统默认租户' } })).id;
  }

  private async validateGatewayRelation(actor: AuthUser, deviceType: string, gatewayId?: string | null, currentDeviceId?: string) {
    if (deviceType !== 'GATEWAY_CHILD') return;
    if (!gatewayId) throw new BadRequestException('网关子设备必须选择所属网关');
    if (gatewayId === currentDeviceId) throw new BadRequestException('设备不能作为自己的上级网关');
    const gateway = await this.getAccessibleDevice(actor, gatewayId);
    if (gateway.deviceType !== 'GATEWAY') throw new BadRequestException('所属网关必须是网关设备');
  }

  private async ensureGatewayCanChangeType(deviceId: string, currentDeviceType: string, nextDeviceType: string) {
    if (currentDeviceType !== 'GATEWAY' || nextDeviceType === 'GATEWAY') return;
    const childCount = await this.prisma.device.count({ where: { gatewayId: deviceId } });
    if (childCount > 0) throw new BadRequestException('该网关下仍有子设备，不能改为非网关设备');
  }

  private async resolveTemplateMetrics(actor: AuthUser, options: { templateDeviceId?: string; modelTemplateId?: string }) {
    if (options.modelTemplateId) {
      const template = await this.prisma.deviceModelTemplate.findUnique({
        where: { id: options.modelTemplateId },
        include: { metrics: true },
      });
      if (!template) throw new BadRequestException('物模型模板不存在');
      if (actor.role !== 'SUPER_ADMIN' && template.tenantId !== actor.tenantId) throw new ForbiddenException('无权使用该物模型模板');
      return template.metrics;
    }
    if (options.templateDeviceId) {
      return this.prisma.deviceMetric.findMany({ where: { deviceId: (await this.getAccessibleDevice(actor, options.templateDeviceId)).id } });
    }
    return [];
  }

  private async resolveImportMetrics(actor: AuthUser, targetDeviceId: string, dto: ImportDeviceMetricsDto) {
    if (dto.modelTemplateId) {
      const template = await this.prisma.deviceModelTemplate.findUnique({
        where: { id: dto.modelTemplateId },
        include: { metrics: true },
      });
      if (!template) throw new BadRequestException('物模型模板不存在');
      if (actor.role !== 'SUPER_ADMIN' && template.tenantId !== actor.tenantId) throw new ForbiddenException('无权使用该物模型模板');
      return template.metrics;
    }
    if (dto.templateDeviceId) {
      const source = await this.getAccessibleDevice(actor, dto.templateDeviceId);
      if (source.id === targetDeviceId) throw new BadRequestException('不能从当前设备导入物模型');
      return this.prisma.deviceMetric.findMany({ where: { deviceId: source.id } });
    }
    throw new BadRequestException('请选择物模型模板或已有设备');
  }

  private sanitizeDeviceKey(deviceKey: string) {
    return deviceKey.replace(/[^a-zA-Z0-9_-]/g, '_');
  }

  private generateMqttPassword() {
    return randomBytes(18).toString('base64url');
  }

  private async generateMqttCredentials(deviceKey: string) {
    const safeKey = this.sanitizeDeviceKey(deviceKey);
    const suffix = randomBytes(4).toString('hex');
    const password = this.generateMqttPassword();
    return {
      clientId: `wk_${safeKey}_${suffix}`,
      username: `device:${safeKey}_${suffix}`,
      password,
      passwordHash: await bcrypt.hash(password, 10),
    };
  }

  private mqttConnectionInfo(device: { deviceKey: string; mqttClientId: string; mqttUsername: string; mqttPasswordUpdatedAt: Date }, password?: string) {
    // 密码只在创建设备或重置时返回一次；平时详情接口不会泄露明文密码。
    return {
      host: process.env.MQTT_PUBLIC_HOST ?? '127.0.0.1',
      port: Number(process.env.MQTT_PUBLIC_PORT ?? 1883),
      wsPort: Number(process.env.MQTT_WS_PORT ?? 8083),
      clientId: device.mqttClientId,
      username: device.mqttUsername,
      password,
      passwordUpdatedAt: device.mqttPasswordUpdatedAt,
      heartbeatTopic: `weikong/devices/${device.deviceKey}/heartbeat`,
      telemetryTopic: `weikong/devices/${device.deviceKey}/telemetry`,
    };
  }

  private toPublicDevice<T extends { mqttPasswordHash?: string }>(device: T) {
    const { mqttPasswordHash: _mqttPasswordHash, ...publicDevice } = device;
    return publicDevice;
  }

  private async getAccessibleDevice(actor: AuthUser, id: string) {
    const device = await this.prisma.device.findUnique({
      where: { id },
      include: {
        tenant: { select: { id: true, name: true } },
        gateway: { select: { id: true, name: true, deviceKey: true, status: true, lastSeenAt: true } },
        children: {
          select: {
            id: true,
            name: true,
            deviceKey: true,
            status: true,
            location: true,
            lastSeenAt: true,
          },
          orderBy: { createdAt: 'desc' },
        },
      },
    });
    if (!device) throw new NotFoundException('设备不存在');
    if (actor.role !== 'SUPER_ADMIN' && device.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该设备');
    return device;
  }

  private telemetryDeviceKeys(device: { deviceKey: string; deviceType: string; children?: Array<{ deviceKey: string }> }) {
    if (device.deviceType !== 'GATEWAY') return [device.deviceKey];
    return [device.deviceKey, ...(device.children ?? []).map((child) => child.deviceKey)];
  }

  private metricDeviceIds(device: { id: string; deviceType: string; children?: Array<{ id: string }> }) {
    if (device.deviceType !== 'GATEWAY') return [device.id];
    return [device.id, ...(device.children ?? []).map((child) => child.id)];
  }
}

import { BadRequestException, ForbiddenException, Injectable, MessageEvent, NotFoundException, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { isUUID } from 'class-validator';
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
import { gatewayPointAccess } from '../common/gateway-point-access';
import { MqttConnectionAdminService } from '../mqtt/mqtt-connection-admin.service';
import { UpdateDeviceCloudAccessDto } from './dto/update-device-cloud-access.dto';
import { RemoteAccessGateway } from '../remote-access/remote-access.gateway';
import { deviceAccessWhere } from '../common/user-permissions';
import { SubscriptionLimitsService } from '../subscriptions/subscription-limits.service';

@Injectable()
export class DevicesService implements OnModuleDestroy {
  // TimescaleDB 专门存储遥测时序数据，避免把高频上报压到业务库里。
  private readonly timescale = new Pool({
    connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong@localhost:5433/weikong_ts',
  });
  // 业务库连接用于 LISTEN/NOTIFY 监听设备状态变化，Prisma 本身不提供持久监听能力。
  private readonly postgres = new Pool({
    connectionString: (process.env.DATABASE_URL ?? 'postgresql://weikong@localhost:5432/weikong_iot?schema=public').replace('?schema=public', ''),
  });
  // API 内存事件总线：数据库通知进来后再转成 SSE 推给浏览器。
  private readonly telemetryUpdates = new Subject<{ deviceId: string; deviceKey: string; time: string; metrics: Record<string, unknown> }>();
  // Internal cloud rules consume only live ingest notifications, never historical snapshots.
  liveTelemetryUpdates() { return this.telemetryUpdates.asObservable(); }
  private readonly statusUpdates = new Subject<{ id: string; tenantId: string; deviceKey: string; name: string; status: string; lastSeenAt?: Date | null }>();
  private telemetryListener?: PoolClient;
  private statusListener?: PoolClient;
  private destroying = false;

  constructor(
    private readonly prisma: PrismaService,
    private readonly mqttConnectionAdmin: MqttConnectionAdminService,
    private readonly remoteAccessGateway: RemoteAccessGateway,
    private readonly subscriptionLimits: SubscriptionLimitsService,
  ) {
    void this.listenForTelemetry();
    void this.listenForDeviceStatus();
  }

  async findAll(
    actor: AuthUser,
    page: number,
    pageSize: number,
    keyword: string,
    deviceType?: string,
    tenantId?: string,
    status?: string,
    protocol?: string,
    groupId?: string,
  ) {
    if (actor.role !== 'SUPER_ADMIN' && !actor.tenantId) throw new ForbiddenException('当前账号未关联租户');
    if (groupId && groupId !== 'ungrouped' && !isUUID(groupId)) throw new BadRequestException('设备分组参数无效');
    const groupWhere: Prisma.DeviceWhereInput = groupId
      ? groupId === 'ungrouped'
        ? { groupMemberships: { none: {} } }
        : { groupMemberships: { some: { groupId } } }
      : {};
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const safeDeviceType = ['GATEWAY', 'GATEWAY_CHILD', 'DIRECT'].includes(deviceType ?? '') ? deviceType : undefined;
    const safeStatus = ['ONLINE', 'OFFLINE', 'DISABLED'].includes(status ?? '') ? status : undefined;
    const safeProtocol = protocol?.trim().slice(0, 32) || undefined;
    const scopeWhere: Prisma.DeviceWhereInput = actor.role === 'SUPER_ADMIN'
      ? tenantId?.trim() ? { tenantId: tenantId.trim() } : {}
      : deviceAccessWhere(actor);
    const commonFilters: Prisma.DeviceWhereInput = {
      ...groupWhere,
      ...(safeDeviceType ? { deviceType: safeDeviceType } : {}),
      ...(safeProtocol ? { protocol: { equals: safeProtocol, mode: 'insensitive' } } : {}),
    };
    const where: Prisma.DeviceWhereInput = {
      AND: [
        scopeWhere,
        commonFilters,
        ...(safeStatus === 'DISABLED'
          ? [{ OR: [{ status: 'DISABLED' }, { tenant: { deviceCloudAccessDisabled: true } }] } as Prisma.DeviceWhereInput]
          : safeStatus ? [{ status: safeStatus, tenant: { deviceCloudAccessDisabled: false } } as Prisma.DeviceWhereInput] : []),
        ...(keyword ? [{ OR: [{ name: { contains: keyword, mode: 'insensitive' } }, { deviceKey: { contains: keyword, mode: 'insensitive' } }] } as Prisma.DeviceWhereInput] : []),
      ],
    };
    const statsWhere: Prisma.DeviceWhereInput = { AND: [scopeWhere, commonFilters] };
    const [items, total, statsTotal, onlineTotal, offlineTotal, disabledTotal] = await this.prisma.$transaction([
      this.prisma.device.findMany({
        where,
        include: {
          groupMemberships: { select: { group: { select: { id: true, name: true } } }, orderBy: { createdAt: 'asc' } },
          tenant: { select: { id: true, name: true, deviceCloudAccessDisabled: true } },
          gateway: { select: { id: true, name: true, deviceKey: true, status: true, cloudAccessEnabled: true, lastSeenAt: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.device.count({ where }),
      this.prisma.device.count({ where: statsWhere }),
      this.prisma.device.count({ where: { ...statsWhere, status: 'ONLINE', tenant: { deviceCloudAccessDisabled: false } } }),
      this.prisma.device.count({ where: { ...statsWhere, status: 'OFFLINE', tenant: { deviceCloudAccessDisabled: false } } }),
      this.prisma.device.count({ where: { ...statsWhere, OR: [{ status: 'DISABLED' }, { tenant: { deviceCloudAccessDisabled: true } }] } }),
    ]);
    return {
      items: items.map((item) => this.toPublicDevice(item)),
      total,
      page: safePage,
      pageSize: safePageSize,
      stats: {
        total: statsTotal,
        online: onlineTotal,
        offline: offlineTotal,
        disabled: disabledTotal,
      },
    };
  }

  async create(actor: AuthUser, dto: CreateDeviceDto) {
    const tenantId = actor.role === 'SUPER_ADMIN' ? await this.getDefaultTenantId() : actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('设备必须指定所属租户');
    const { templateDeviceId, modelTemplateId, ...deviceData } = dto;
    const deviceType = deviceData.deviceType ?? 'DIRECT';
    const gatewayId = deviceType === 'GATEWAY_CHILD' ? deviceData.gatewayId : null;
    await this.validateGatewayRelation(actor, deviceType, gatewayId);
    await this.ensureDeviceKeyAvailable(dto.deviceKey, gatewayId);
    const credentials = await this.generateMqttCredentials(dto.deviceKey);
    // 物模型模板只在创建设备时复制一份，后续设备字段可以独立调整。
    const templateMetrics = await this.resolveTemplateMetrics(actor, { templateDeviceId, modelTemplateId });
    await this.subscriptionLimits.assertCanAddDevices(tenantId);
    if (templateMetrics.length) await this.subscriptionLimits.assertCanAddPoints(tenantId, templateMetrics.length);
    return this.prisma.$transaction(async (tx) => {
      const device = await tx.device.create({
        data: {
          ...deviceData,
          tenantId,
          deviceType,
          gatewayId,
          status: 'OFFLINE',
          mqttClientId: credentials.clientId,
          mqttUsername: credentials.username,
          mqttPasswordHash: credentials.passwordHash,
          mqttPasswordUpdatedAt: new Date(),
        },
      });
      if (actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM') {
        await tx.userDeviceAccess.create({ data: { userId: actor.sub, deviceId: device.id } });
      }
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
          gateway: { select: { id: true, name: true, deviceKey: true, status: true, cloudAccessEnabled: true, lastSeenAt: true } },
          groupMemberships: { select: { group: { select: { id: true, name: true } } }, orderBy: { createdAt: 'asc' } },
        },
      });
      return created ? { ...this.toPublicDevice(created), mqtt: this.mqttConnectionInfo(created, credentials.password) } : null;
    });
  }

  async modelTemplates(actor: AuthUser) {
    return this.prisma.device.findMany({
      where: {
        ...deviceAccessWhere(actor),
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
        ...deviceAccessWhere(actor),
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
    const nextGatewayId = nextDeviceType === 'GATEWAY_CHILD' ? (deviceData.gatewayId ?? device.gatewayId) : null;
    const nextDeviceKey = deviceData.deviceKey ?? device.deviceKey;
    await this.validateGatewayRelation(actor, nextDeviceType, nextGatewayId, device.id);
    await this.ensureGatewayCanChangeType(device.id, device.deviceType, nextDeviceType);
    await this.ensureDeviceKeyAvailable(nextDeviceKey, nextGatewayId, device.id);
    const updateData: Prisma.DeviceUncheckedUpdateInput = {
      ...deviceData,
      deviceType: nextDeviceType,
      gatewayId: nextGatewayId,
    };
    const updated = await this.prisma.device.update({
      where: { id: device.id },
      data: updateData,
      include: {
        tenant: { select: { id: true, name: true } },
        gateway: { select: { id: true, name: true, deviceKey: true, status: true, cloudAccessEnabled: true, lastSeenAt: true } },
        groupMemberships: { select: { group: { select: { id: true, name: true } } }, orderBy: { createdAt: 'asc' } },
      },
    });
    return this.toPublicDevice(updated);
  }

  async remove(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    if (actor.role !== 'SUPER_ADMIN' && !device.cloudAccessEnabled) {
      throw new ForbiddenException('该设备已被管理平台禁用，请联系平台管理员解禁或移除');
    }
    return this.prisma.$transaction(async (tx) => {
      if (device.deviceType === 'GATEWAY_CHILD' && device.gatewayId && !device.cloudAccessEnabled) {
        await tx.gatewayChildCloudRestriction.upsert({
          where: {
            gatewayDeviceId_childDeviceKey: {
              gatewayDeviceId: device.gatewayId,
              childDeviceKey: device.deviceKey,
            },
          },
          create: {
            gatewayDeviceId: device.gatewayId,
            childDeviceKey: device.deviceKey,
            disabledAt: device.cloudDisabledAt ?? new Date(),
            disabledBy: device.cloudDisabledBy ?? actor.sub,
            reason: device.cloudDisabledReason ?? '管理平台禁用',
          },
          update: {},
        });
      }
      const factoryGateway = await tx.factoryGateway.findUnique({
        where: { deviceId: device.id },
        select: { id: true, sn: true, hardwareId: true },
      });
      if (factoryGateway) {
        await tx.factoryGateway.update({
          where: { id: factoryGateway.id },
          data: { status: 'UNBOUND', deviceId: null, boundAt: null },
        });
      }
      await tx.device.delete({ where: { id: device.id } });
      return {
        message: factoryGateway ? '设备已删除，出厂网关已解除绑定' : '删除成功',
        factoryGateway,
      };
    });
  }

  async reportStatus(actor: AuthUser, id: string, dto: ReportDeviceStatusDto) {
    const device = await this.getAccessibleDevice(actor, id);
    if (!device.cloudAccessEnabled && dto.status === 'ONLINE') {
      throw new ForbiddenException('该设备的云连接已被管理平台禁用');
    }
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

  async updateCloudAccess(actor: AuthUser, id: string, dto: UpdateDeviceCloudAccessDto) {
    if (actor.role !== 'SUPER_ADMIN') throw new ForbiddenException('仅平台超级管理员可以调整设备云连接权限');
    const device = await this.getAccessibleDevice(actor, id);
    const reason = dto.enabled ? null : dto.reason?.trim() || '管理平台禁用';
    if (device.cloudAccessEnabled === dto.enabled) {
      if (device.deviceType === 'GATEWAY_CHILD' && device.gatewayId) {
        if (dto.enabled) {
          await this.prisma.gatewayChildCloudRestriction.deleteMany({
            where: { gatewayDeviceId: device.gatewayId, childDeviceKey: device.deviceKey },
          });
        } else {
          await this.prisma.gatewayChildCloudRestriction.upsert({
            where: {
              gatewayDeviceId_childDeviceKey: {
                gatewayDeviceId: device.gatewayId,
                childDeviceKey: device.deviceKey,
              },
            },
            create: {
              gatewayDeviceId: device.gatewayId,
              childDeviceKey: device.deviceKey,
              disabledAt: device.cloudDisabledAt ?? new Date(),
              disabledBy: device.cloudDisabledBy ?? actor.sub,
              reason: device.cloudDisabledReason ?? reason,
            },
            update: {},
          });
        }
      }
      return { device: this.toPublicDevice(device), disconnectResults: [], connectionPolicyChanged: false };
    }
    const disabledAt = new Date();
    const updated = await this.prisma.$transaction(async (tx) => {
      if (device.deviceType === 'GATEWAY_CHILD' && device.gatewayId) {
        if (dto.enabled) {
          await tx.gatewayChildCloudRestriction.deleteMany({
            where: { gatewayDeviceId: device.gatewayId, childDeviceKey: device.deviceKey },
          });
        } else {
          await tx.gatewayChildCloudRestriction.upsert({
            where: {
              gatewayDeviceId_childDeviceKey: {
                gatewayDeviceId: device.gatewayId,
                childDeviceKey: device.deviceKey,
              },
            },
            create: {
              gatewayDeviceId: device.gatewayId,
              childDeviceKey: device.deviceKey,
              disabledAt,
              disabledBy: actor.sub,
              reason,
            },
            update: { disabledAt, disabledBy: actor.sub, reason },
          });
        }
      }
      return tx.device.update({
        where: { id: device.id },
        data: dto.enabled
          ? {
              cloudAccessEnabled: true,
              cloudDisabledAt: null,
              cloudDisabledBy: null,
              cloudDisabledReason: null,
              status: 'OFFLINE',
            }
          : {
              cloudAccessEnabled: false,
              cloudDisabledAt: disabledAt,
              cloudDisabledBy: actor.sub,
              cloudDisabledReason: reason,
              status: 'DISABLED',
            },
        include: {
          tenant: { select: { id: true, name: true } },
          gateway: { select: { id: true, name: true, deviceKey: true, status: true, cloudAccessEnabled: true, lastSeenAt: true } },
          factoryGateway: { select: { hardwareId: true, status: true } },
        },
      });
    });

    const disconnectResults: Array<{ clientId: string; result: string }> = [];
    if (!dto.enabled && device.deviceType !== 'GATEWAY_CHILD') {
      this.remoteAccessGateway.disconnectDevice(device.id);
      const clientIds = new Set<string>([device.mqttClientId]);
      if (device.factoryGateway?.hardwareId) clientIds.add(`factory_${device.factoryGateway.hardwareId}`);
      for (const clientId of clientIds) {
        disconnectResults.push({ clientId, result: await this.mqttConnectionAdmin.disconnect(clientId) });
      }
    }
    if (!dto.enabled && device.deviceType === 'GATEWAY') {
      const children = await this.prisma.device.findMany({
        where: { gatewayId: device.id, status: { not: 'DISABLED' } },
        select: { id: true, tenantId: true, deviceKey: true, name: true },
      });
      if (children.length) {
        await this.prisma.device.updateMany({
          where: { id: { in: children.map((child) => child.id) }, status: { not: 'DISABLED' } },
          data: { status: 'OFFLINE' },
        });
        for (const child of children) this.statusUpdates.next({ ...child, status: 'OFFLINE' });
      }
    }
    this.statusUpdates.next(updated);
    return {
      device: this.toPublicDevice(updated),
      disconnectResults,
      connectionPolicyChanged: true,
      sharedGatewayConnectionRetained: !dto.enabled && device.deviceType === 'GATEWAY_CHILD',
    };
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
      include: {
        tenant: { select: { id: true, name: true } },
        gateway: { select: { deviceKey: true } },
        factoryGateway: { select: { hardwareId: true, status: true } },
      },
    });
    return this.mqttConnectionInfo(updated, password);
  }

  async telemetry(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceIds = this.telemetryDeviceIds(device);
    const result = await this.timescale.query<{ time: Date; deviceId: string; deviceKey: string; metrics: Record<string, unknown> }>(
      'SELECT time, device_id as "deviceId", device_key as "deviceKey", metrics FROM telemetry_events WHERE device_id = ANY($1) AND time >= $2 AND time <= $3 ORDER BY time DESC LIMIT 20',
      [deviceIds, (await this.subscriptionLimits.history(device.tenantId)).queryEarliestAt, new Date()],
    );
    return { deviceKey: device.deviceKey, items: result.rows };
  }

  historyWindow(actor: AuthUser) {
    if (!actor.tenantId) throw new BadRequestException('历史保存范围需在租户上下文中查询');
    return this.subscriptionLimits.history(actor.tenantId);
  }

  async exportHistory(actor: AuthUser, pairs: Array<{ value1?: string; value2?: string }>, start?: string, end?: string, keyword?: string, numeric?:{min?:number;max?:number}) {
    // Fix the end time across all pages; authorization and retention are rechecked each page.
    const fixedEnd = end || new Date().toISOString();
    const fixedStart = start || new Date(new Date(fixedEnd).getTime() - 3_600_000).toISOString();
    const rows: Record<string, unknown>[] = [];
    let columns: Array<{ title: string; key: string }> = [], total = 0;
    for (let page = 1; page <= 20; page++) {
      const result = await this.telemetryHistoryTable(actor, pairs, fixedStart, fixedEnd, page, 100, keyword, false, numeric);
      columns = result.dataset.dimensions; total = result.total;
      rows.push(...result.dataset.source);
      if (!result.hasNext) break;
    }
    const csvValue = (value: unknown) => {
      const text = value == null ? '' : String(value);
      const safe = typeof value === 'string' && /^[\s]*[=+@-]/.test(text) ? `'${text}` : text;
      return `"${safe.replaceAll('"', '""')}"`;
    };
    const csv = '\uFEFF' + [columns.map(c => csvValue(c.title)).join(','), ...rows.map(r => columns.map(c => csvValue(r[c.key])).join(','))].join('\r\n');
    return { csv, total, exported: rows.length, limited: total > rows.length, maxExportRows: 2000 };
  }

  async telemetryTrend(actor: AuthUser, id: string, metric: string, range: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceIds = this.telemetryDeviceIds(device);
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
        WHERE device_id = ANY($3)
          AND time >= NOW() - $4::interval
          AND time >= $5 AND time <= NOW()
          AND metrics ? $2
          AND jsonb_typeof(metrics -> $2) = 'number'
        GROUP BY 1
        ORDER BY 1 ASC
      `,
      [option.bucket, metricKey, deviceIds, option.interval, (await this.subscriptionLimits.history(device.tenantId)).queryEarliestAt],
    );
    return {
      deviceKey: device.deviceKey,
      metric: metricKey,
      range: option.key,
      bucket: option.bucket,
      items: result.rows.map((row) => ({ time: row.time, value: Number(row.value) })),
    };
  }

  async telemetryHistoryTable(
    actor: AuthUser,
    pairs: Array<{ value1?: string; value2?: string }>,
    start?: string,
    end?: string,
    page?: number,
    pageSize?: number,
    keyword?: string,
    chart = false,
    numeric?:{min?:number;max?:number},
  ) {
    if ([numeric?.min,numeric?.max].some(v=>v!==undefined&&!Number.isFinite(v)) || (numeric?.min!==undefined&&numeric?.max!==undefined&&numeric.min>numeric.max)) throw new BadRequestException('数值筛选范围无效');
    if (!Array.isArray(pairs)) throw new BadRequestException('点位列表格式无效');
    if (pairs.length > 20) throw new BadRequestException('单次最多查询20个点位');
    const normalizedPairs = pairs
      .map(pair => ({ deviceId: pair.value1?.trim() ?? '', metric: pair.value2?.trim() ?? '' }))
      .filter(pair => pair.deviceId && pair.metric)
      .slice(0, 20);
    if (!normalizedPairs.length) throw new BadRequestException('请至少选择一个设备点位');
    if (normalizedPairs.some(pair => !isUUID(pair.deviceId) || !/^[\w.\-:]+$/.test(pair.metric))) {
      throw new BadRequestException('设备或点位参数无效');
    }

    const parseHistoryDate = (value: string) => {
      const normalized = /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}$/.test(value)
        ? `${value.replace(' ', 'T')}+08:00`
        : value;
      return new Date(normalized);
    };
    const endAt = end ? parseHistoryDate(end) : new Date();
    const startAt = start ? parseHistoryDate(start) : new Date(endAt.getTime() - 60 * 60 * 1000);
    if (endAt.getTime() > Date.now() + 1000) throw new BadRequestException('不能查询未来时间');
    if (Number.isNaN(startAt.getTime()) || Number.isNaN(endAt.getTime()) || startAt >= endAt) {
      throw new BadRequestException('历史数据时间范围无效');
    }
    if (endAt.getTime() - startAt.getTime() > 31 * 24 * 60 * 60 * 1000) {
      throw new BadRequestException('单次历史查询不能超过 31 天');
    }

    const requestedDeviceIds = [...new Set(normalizedPairs.map(pair => pair.deviceId))];
    const devices = await this.prisma.device.findMany({
      where: { AND: [deviceAccessWhere(actor), { id: { in: requestedDeviceIds } }] },
      select: {
        id: true,
        name: true,
        tenantId: true,
        metrics: {
          where: { enabled: true, ignored: false },
          select: { identifier: true, name: true, unit: true },
        },
      },
    });
    if (devices.length !== requestedDeviceIds.length) throw new ForbiddenException('部分设备不存在或无权访问');

    const deviceMap = new Map(devices.map(device => [device.id, device]));
    const tenantWindows = new Map(await Promise.all([...new Set(devices.map(d => d.tenantId))].map(async tenantId => [tenantId, await this.subscriptionLimits.history(tenantId)] as const)));
    const retentionRows = devices.map(d => ({ deviceId: d.id, earliestAt: tenantWindows.get(d.tenantId)!.queryEarliestAt.toISOString() }));
    const earliestAt = new Date(Math.max(...[...tenantWindows.values()].map(r => r.queryEarliestAt.getTime())));
    if (startAt < earliestAt) throw new BadRequestException(`超出历史保存期限，当前所选设备共同可查询范围从${earliestAt.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })}开始，请调整时间范围`);
    const invalidPoints = normalizedPairs.filter(p => !deviceMap.get(p.deviceId)!.metrics.some(m => m.identifier === p.metric));
    if (invalidPoints.length) throw new BadRequestException(`以下点位不存在、已停用或未开放云端历史查询：${invalidPoints.map(p => `${deviceMap.get(p.deviceId)!.name} / ${p.metric}`).join('、')}`);
    const paginated = page !== undefined || pageSize !== undefined;
    const safePage = Math.min(1_000_000, Math.max(1, Number.isFinite(Number(page)) ? Math.floor(Number(page)) : 1));
    const safePageSize = paginated
      ? Math.min(100, Math.max(1, Number.isFinite(Number(pageSize)) ? Math.floor(Number(pageSize)) : 10))
      : 500;
    const offset = (safePage - 1) * safePageSize;
    const safeKeyword = String(keyword ?? '').trim().slice(0, 100);
    const keywordDeviceIds = safeKeyword
      ? devices.filter(device => device.name.toLowerCase().includes(safeKeyword.toLowerCase())).map(device => device.id)
      : [];
    const requestedPairs = JSON.stringify(normalizedPairs.map(pair => ({ deviceId: pair.deviceId, metric: pair.metric, min:numeric?.min, max:numeric?.max })));
    const queryParams = [requestedDeviceIds, startAt, endAt, requestedPairs, safeKeyword, keywordDeviceIds, JSON.stringify(retentionRows)];
    const matchingRows = `
      FROM telemetry_events AS event
      WHERE event.device_id = ANY($1)
        AND event.time >= $2
        AND event.time <= $3
        AND EXISTS (SELECT 1 FROM jsonb_to_recordset($7::jsonb) AS retention("deviceId" text, "earliestAt" timestamptz)
          WHERE retention."deviceId" = event.device_id AND event.time >= retention."earliestAt")
        AND EXISTS (
          SELECT 1
          FROM jsonb_to_recordset($4::jsonb) AS requested("deviceId" text, metric text, min double precision, max double precision)
          WHERE requested."deviceId" = event.device_id
            AND event.metrics ? requested.metric
            AND ((requested.min IS NULL AND requested.max IS NULL) OR
              CASE WHEN jsonb_typeof(event.metrics -> requested.metric) = 'number' THEN
                (requested.min IS NULL OR (event.metrics ->> requested.metric)::double precision >= requested.min)
                AND (requested.max IS NULL OR (event.metrics ->> requested.metric)::double precision <= requested.max)
              ELSE false END)
        )
        AND (
          $5::text = ''
          OR event.device_id = ANY($6::text[])
          OR EXISTS (
            SELECT 1
            FROM jsonb_to_recordset($4::jsonb) AS searched("deviceId" text, metric text)
            WHERE searched."deviceId" = event.device_id
              AND COALESCE(event.metrics ->> searched.metric, '') ILIKE '%' || $5 || '%'
          )
        )
    `;
    const bucketSeconds = Math.max(1, Math.ceil((endAt.getTime() - startAt.getTime()) / 1000 / Math.max(1, Math.floor(500 / requestedDeviceIds.length))));
    const chartFrom = `FROM (
      SELECT bucket AS time, "deviceId", jsonb_object_agg(metric, value) AS metrics FROM (
        SELECT time_bucket($8::interval, event.time) AS bucket, event.device_id AS "deviceId", point.metric,
          AVG((event.metrics ->> point.metric)::double precision) AS value
        ${matchingRows.replace('FROM telemetry_events AS event', 'FROM telemetry_events AS event CROSS JOIN jsonb_to_recordset($4::jsonb) AS point("deviceId" text, metric text)')}
        AND point."deviceId" = event.device_id AND jsonb_typeof(event.metrics -> point.metric) = 'number'
        GROUP BY bucket, event.device_id, point.metric
      ) samples GROUP BY bucket, "deviceId"
    ) chart`;
    const countParams = chart ? [...queryParams, `${bucketSeconds} seconds`] : queryParams;
    const [result, countResult] = await Promise.all([
      this.timescale.query<{ time: Date; deviceId: string; metrics: Record<string, unknown> }>(
      `
        ${chart ? 'SELECT time, "deviceId", metrics' : 'SELECT event.time, event.device_id AS "deviceId", event.metrics'}
        ${chart ? chartFrom : matchingRows}
        ORDER BY ${chart ? 'time DESC, "deviceId" ASC' : 'event.time DESC, event.device_id ASC'}
        LIMIT $${chart ? 9 : 8} OFFSET $${chart ? 10 : 9}
      `,
        [...countParams, safePageSize, offset],
      ),
      this.timescale.query<{ total: string }>(`SELECT COUNT(*)::text AS total ${chart ? chartFrom : matchingRows}`, countParams),
    ]);
    const total = Number(countResult.rows[0]?.total ?? 0);

    const columns = normalizedPairs.map((pair, index) => {
      const device = deviceMap.get(pair.deviceId)!;
      const metric = device.metrics.find(item => item.identifier === pair.metric);
      const unit = metric?.unit ? ` (${metric.unit})` : '';
      return {
        title: `${device.name} / ${metric?.name || pair.metric}${unit}`,
        key: `metric_${index}`,
        deviceId: pair.deviceId,
        metric: pair.metric,
      };
    });
    const timeFormatter = new Intl.DateTimeFormat('zh-CN', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
    });
    const source = result.rows.map((row, rowIndex) => {
      const record: Record<string, unknown> = {
        key: rowIndex,
        timestamp: timeFormatter.format(new Date(row.time)).replaceAll('/', '-'),
        deviceName: deviceMap.get(row.deviceId)?.name || row.deviceId,
      };
      columns.forEach(column => {
        record[column.key] = column.deviceId === row.deviceId ? row.metrics?.[column.metric] ?? null : null;
      });
      return record;
    });
    return {
      dataset: {
        dimensions: [
          { title: '时间', key: 'timestamp' },
          { title: '设备', key: 'deviceName' },
          ...columns.map(({ title, key }) => ({ title, key })),
        ],
        source,
      },
      total,
      retention: { earliestAt, latestAt: new Date(), maxQueryDays: 31, cleanupGraceDays: 7 },
      aggregation: chart ? { method: 'AVG', bucketSeconds, description: `按${bucketSeconds}秒时间桶取平均值；缺失值不补0` } : null,
      page: safePage,
      pageSize: safePageSize,
      totalPages: total === 0 ? 0 : Math.ceil(total / safePageSize),
      hasNext: offset + source.length < total,
      limited: !paginated && total > safePageSize,
    };
  }

  async telemetryStream(actor: AuthUser, id: string): Promise<Observable<MessageEvent>> {
    const device = await this.getAccessibleDevice(actor, id);
    const gatewayId = device.deviceType === 'GATEWAY' ? device.id : null;
    return new Observable<MessageEvent>((subscriber) => {
      let deviceIds = new Set(this.telemetryDeviceIds(device));
      let refreshing = false;
      let closed = false;
      const refresh = async () => {
        if (refreshing || closed) return;
        refreshing = true;
        try {
          const accessible = await this.prisma.device.findMany({
            where: {
              AND: [
                deviceAccessWhere(actor),
                gatewayId ? { OR: [{ id: gatewayId }, { gatewayId }] } : { id: device.id },
              ],
            },
            select: { id: true },
          });
          deviceIds = new Set(accessible.map(item => item.id));
        } finally {
          refreshing = false;
        }
      };
      const liveSubscription = this.telemetryUpdates.pipe(
        filter(event => deviceIds.has(event.deviceId)),
        map(event => ({ data: event })),
      ).subscribe(subscriber);
      const refreshTimer = setInterval(() => { void refresh().catch(error => subscriber.error(error)); }, 3000);
      return () => {
        closed = true;
        clearInterval(refreshTimer);
        liveSubscription.unsubscribe();
      };
    });
  }

  statusStream(actor: AuthUser): Observable<MessageEvent> {
    return new Observable<MessageEvent>((subscriber) => {
      let liveSubscription: { unsubscribe: () => void } | undefined;
      let refreshTimer: ReturnType<typeof setInterval> | undefined;
      let refreshing = false;
      let closed = false;
      let accessibleIds = new Set<string>();
      const refreshAccess = async () => {
        if (actor.role === 'SUPER_ADMIN' || refreshing || closed) return;
        refreshing = true;
        try {
          const devices = await this.prisma.device.findMany({ where: deviceAccessWhere(actor), select: { id: true } });
          accessibleIds = new Set(devices.map(device => device.id));
        } finally {
          refreshing = false;
        }
      };
      // SSE 建连后先推一次快照，避免页面刷新后要等下一次设备状态变化才有数据。
      void this.statusSnapshot(actor)
        .then((devices) => {
          accessibleIds = new Set(devices.map(device => device.id));
          for (const device of devices) subscriber.next({ data: { ...device, snapshot: true } });
          liveSubscription = this.statusUpdates.pipe(
            filter((device) => actor.role === 'SUPER_ADMIN' || accessibleIds.has(device.id)),
            map((device) => ({ data: device })),
          ).subscribe(subscriber);
          if (actor.role !== 'SUPER_ADMIN') {
            refreshTimer = setInterval(() => { void refreshAccess().catch(error => subscriber.error(error)); }, 3000);
          }
        })
        .catch((error) => subscriber.error(error));
      return () => {
        closed = true;
        if (refreshTimer) clearInterval(refreshTimer);
        liveSubscription?.unsubscribe();
      };
    });
  }

  async metrics(actor: AuthUser, id: string) {
    const device = await this.getAccessibleDevice(actor, id);
    const deviceIds = this.metricDeviceIds(device);
    const metrics = await this.prisma.deviceMetric.findMany({
      where: { deviceId: { in: deviceIds } },
      orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }],
    });
    if (device.deviceType === 'GATEWAY') {
      const owners = new Map([device, ...device.children].map(d => [d.id, d.name]));
      return metrics.map(metric => ({ ...metric, deviceName: owners.get(metric.deviceId), isGatewayMetric: metric.deviceId === device.id }));
    }
    if (device.deviceType !== 'GATEWAY_CHILD' || !device.gatewayId) return metrics;
    const [gateway, draft] = await Promise.all([
      this.prisma.device.findFirst({ where: { id: device.gatewayId, tenantId: device.tenantId, deviceType: 'GATEWAY' }, select: { reportedConfig: true } }),
      this.prisma.gatewayTopologyDraft.findUnique({ where: { gatewayDeviceId: device.gatewayId }, select: { items: true } }),
    ]);
    if (!gateway) return metrics;
    const access = gatewayPointAccess(gateway.reportedConfig, draft?.items, device.deviceKey);
    if (access === null) return metrics;
    // Apply immediately even when telemetry registered a metric after the last topology report.
    return metrics.map(metric => ({ ...metric, dataType: access.find(point => point.metric === metric.identifier)?.dataType || metric.dataType, accessMode: access.some(point => point.metric === metric.identifier && point.canWrite) ? 'READ_WRITE' : 'READ_ONLY', accessSource: 'GATEWAY' }));
  }

  async updateMetric(actor: AuthUser, id: string, metricId: string, dto: UpdateDeviceMetricDto) {
    const device = await this.getAccessibleDevice(actor, id);
    const metric = await this.prisma.deviceMetric.findFirst({ where: { id: metricId, deviceId: { in: this.metricDeviceIds(device) } } });
    if (!metric) throw new NotFoundException('指标不存在');
    const isActive = metric.enabled && !metric.ignored;
    const willBeActive = (dto.enabled ?? metric.enabled) && !(dto.ignored ?? metric.ignored);
    if (!isActive && willBeActive) await this.subscriptionLimits.assertCanAddPoints(device.tenantId, 1);
    const { accessMode, ...displaySettings } = dto;
    return this.prisma.deviceMetric.update({ where: { id: metric.id }, data: device.deviceType === 'GATEWAY_CHILD' ? displaySettings : dto });
  }

  async importMetrics(actor: AuthUser, id: string, dto: ImportDeviceMetricsDto) {
    const target = await this.getAccessibleDevice(actor, id);
    const sourceMetrics = await this.resolveImportMetrics(actor, target.id, dto);
    if (!sourceMetrics.length) throw new BadRequestException('暂无可导入的物模型字段');
    const existingMetrics = await this.prisma.deviceMetric.findMany({ where: { deviceId: target.id }, select: { identifier: true, enabled: true, ignored: true } });
    const existingByIdentifier = new Map(existingMetrics.map((metric) => [metric.identifier, metric]));
    const existingIdentifiers = new Set(existingByIdentifier.keys());
    const newMetricCount = sourceMetrics.filter(metric => {
      const existing = existingByIdentifier.get(metric.identifier);
      if (!existing) return metric.enabled;
      return Boolean(dto.overwrite && metric.enabled && !existing.enabled && !existing.ignored);
    }).length;
    if (newMetricCount) await this.subscriptionLimits.assertCanAddPoints(target.tenantId, newMetricCount);
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
      where: deviceAccessWhere(actor),
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
    const defaultPlan = await this.prisma.subscriptionPlan.findFirst({
      where: { enabled: true },
      orderBy: [{ priceCents: 'asc' }, { sortOrder: 'asc' }, { createdAt: 'asc' }],
      select: { id: true, deviceLimit: true, pointLimit: true, scadaProjectLimit: true, userLimit: true, gracePeriodDays: true },
    });
    const now = new Date();
    const expiresAt = new Date(now); expiresAt.setUTCFullYear(expiresAt.getUTCFullYear() + 1);
    const graceEndsAt = new Date(expiresAt.getTime() + (defaultPlan?.gracePeriodDays ?? 7) * 86_400_000);
    return (await this.prisma.tenant.create({ data: {
      name: '系统默认租户',
      subscriptionPlanId: defaultPlan?.id,
      subscriptionStartedAt: defaultPlan ? new Date() : undefined,
      ...(defaultPlan ? { subscription: { create: {
        planId: defaultPlan.id, status: 'ACTIVE', billingCycle: 'YEARLY', startedAt: now, expiresAt, graceEndsAt,
        baseDeviceLimit: defaultPlan.deviceLimit, basePointLimit: defaultPlan.pointLimit,
        baseScadaProjectLimit: defaultPlan.scadaProjectLimit, baseUserLimit: defaultPlan.userLimit,
      } } } : {}),
    } })).id;
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

  private async ensureDeviceKeyAvailable(deviceKey: string, gatewayId: string | null | undefined, excludeDeviceId?: string) {
    const existing = await this.prisma.device.findFirst({
      where: {
        deviceKey,
        gatewayId: gatewayId ?? null,
        ...(!gatewayId ? { deviceType: { not: 'GATEWAY_CHILD' } } : {}),
        ...(excludeDeviceId ? { id: { not: excludeDeviceId } } : {}),
      },
      select: { id: true },
    });
    if (!existing) return;
    throw new BadRequestException(gatewayId ? '该网关下的设备编号已存在' : '直连设备或网关编号已存在');
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

  private mqttConnectionInfo(device: {
    deviceKey: string;
    deviceType?: string;
    gateway?: { deviceKey: string } | null;
    factoryGateway?: { hardwareId: string; status: string } | null;
    mqttClientId: string;
    mqttUsername: string;
    mqttPasswordUpdatedAt: Date;
  }, password?: string) {
    // 密码只在创建设备或重置时返回一次；平时详情接口不会泄露明文密码。
    const authMode = device.deviceType === 'GATEWAY_CHILD'
      ? 'GATEWAY_PROXY'
      : device.deviceType === 'GATEWAY' && device.factoryGateway?.status === 'BOUND'
        ? 'FACTORY_ACTIVATION'
        : 'DEVICE_MQTT';
    return {
      host: process.env.MQTT_PUBLIC_HOST ?? '127.0.0.1',
      port: Number(process.env.MQTT_PUBLIC_PORT ?? 1883),
      wsPort: Number(process.env.MQTT_WS_PORT ?? 8083),
      clientId: device.mqttClientId,
      username: device.mqttUsername,
      password,
      passwordUpdatedAt: device.mqttPasswordUpdatedAt,
      authMode,
      activationHardwareId: authMode === 'FACTORY_ACTIVATION' ? device.factoryGateway?.hardwareId : undefined,
      resetAffectsCurrentConnection: authMode === 'DEVICE_MQTT',
      heartbeatTopic: device.deviceType === 'GATEWAY_CHILD' && device.gateway
        ? `weikong/gateways/${device.gateway.deviceKey}/children/${device.deviceKey}/heartbeat`
        : `weikong/devices/${device.deviceKey}/heartbeat`,
      telemetryTopic: device.deviceType === 'GATEWAY_CHILD' && device.gateway
        ? `weikong/gateways/${device.gateway.deviceKey}/children/${device.deviceKey}/telemetry`
        : `weikong/devices/${device.deviceKey}/telemetry`,
    };
  }

  private toPublicDevice<T extends { mqttPasswordHash?: string; factoryGateway?: unknown; groupMemberships?: Array<{ group: { id: string; name: string } }> }>(device: T) {
    const { mqttPasswordHash: _mqttPasswordHash, factoryGateway: _factoryGateway, groupMemberships, ...publicDevice } = device;
    return { ...publicDevice, ...(groupMemberships ? { groups: groupMemberships.map(item => item.group) } : {}) };
  }

  private async getAccessibleDevice(actor: AuthUser, id: string) {
    const device = await this.prisma.device.findFirst({
      where: { AND: [{ id }, deviceAccessWhere(actor)] },
      include: {
        tenant: { select: { id: true, name: true, deviceCloudAccessDisabled: true } },
        gateway: { select: { id: true, name: true, deviceKey: true, status: true, cloudAccessEnabled: true, lastSeenAt: true } },
        factoryGateway: { select: { hardwareId: true, status: true } },
        groupMemberships: { select: { group: { select: { id: true, name: true } } }, orderBy: { createdAt: 'asc' } },
        children: {
          where: deviceAccessWhere(actor),
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
    if (!device) throw new NotFoundException('设备不存在或当前账号无权访问');
    return device;
  }

  private telemetryDeviceIds(device: { id: string; deviceType: string; children?: Array<{ id: string }> }) {
    if (device.deviceType !== 'GATEWAY') return [device.id];
    return [device.id, ...(device.children ?? []).map((child) => child.id)];
  }

  private metricDeviceIds(device: { id: string; deviceType: string; children?: Array<{ id: string }> }) {
    if (device.deviceType !== 'GATEWAY') return [device.id];
    return [device.id, ...(device.children ?? []).map((child) => child.id)];
  }
}

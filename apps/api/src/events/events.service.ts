import { Injectable } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { deviceAccessWhere } from '../common/user-permissions';

interface EventQuery {
  page: number;
  pageSize: number;
  state: string;
  level: string;
  gatewayId: string;
  deviceId: string;
  keyword: string;
  startAt: string;
  endAt: string;
  tenantId: string;
}

@Injectable()
export class EventsService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(actor: AuthUser, query: EventQuery) {
    const page = Math.max(1, Number.isFinite(query.page) ? query.page : 1);
    const pageSize = Math.min(100, Math.max(1, Number.isFinite(query.pageSize) ? query.pageSize : 20));
    const occurredAt = this.dateRange(query.startAt, query.endAt);
    const keyword = query.keyword.trim();
    const tenantId = actor.role === 'SUPER_ADMIN' ? query.tenantId : actor.tenantId;
    // 超级管理员必须明确指定租户；租户账号缺少归属时同样拒绝查询，防止退化为全表读取。
    if (!tenantId) {
      return {
        items: [], total: 0, page, pageSize,
        stateCounts: { ACTIVE: 0, ACKNOWLEDGED: 0, RESOLVED: 0, MANUAL_RESOLVED: 0 },
        gateways: [],
      };
    }
    const baseWhere: Prisma.GatewayEventWhereInput = {
      tenantId,
      ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM'
        ? { AND: [{ OR: [{ device: { is: deviceAccessWhere(actor) } }, { gateway: deviceAccessWhere(actor) }] }] }
        : {}),
      ...(query.gatewayId ? { gatewayId: query.gatewayId } : {}),
      ...(query.deviceId ? { deviceId: query.deviceId } : {}),
      ...(query.level ? { level: query.level } : {}),
      ...(occurredAt ? { occurredAt } : {}),
      ...(keyword ? {
        OR: [
          { message: { contains: keyword, mode: 'insensitive' } },
          { ruleName: { contains: keyword, mode: 'insensitive' } },
          { pointName: { contains: keyword, mode: 'insensitive' } },
          { deviceKey: { contains: keyword, mode: 'insensitive' } },
          { gateway: { name: { contains: keyword, mode: 'insensitive' } } },
          { device: { name: { contains: keyword, mode: 'insensitive' } } },
        ],
      } : {}),
    };
    const where = { ...baseWhere, ...(query.state ? { state: query.state } : {}) };
    const [items, total, activeCount, acknowledgedCount, resolvedCount, manualResolvedCount, gateways] = await this.prisma.$transaction([
      this.prisma.gatewayEvent.findMany({
        where,
        include: {
          gateway: { select: { id: true, deviceKey: true, name: true } },
          device: { select: { id: true, deviceKey: true, name: true } },
          tenant: { select: { id: true, name: true } },
        },
        orderBy: [{ occurredAt: 'desc' }, { createdAt: 'desc' }],
        skip: (page - 1) * pageSize,
        take: pageSize,
      }),
      this.prisma.gatewayEvent.count({ where }),
      this.prisma.gatewayEvent.count({ where: { ...baseWhere, state: 'ACTIVE' } }),
      this.prisma.gatewayEvent.count({ where: { ...baseWhere, state: 'ACKNOWLEDGED' } }),
      this.prisma.gatewayEvent.count({ where: { ...baseWhere, state: 'RESOLVED' } }),
      this.prisma.gatewayEvent.count({ where: { ...baseWhere, state: 'MANUAL_RESOLVED' } }),
      this.prisma.device.findMany({
        where: { AND: [{ deviceType: 'GATEWAY', tenantId }, deviceAccessWhere(actor)] },
        select: { id: true, deviceKey: true, name: true },
        orderBy: { name: 'asc' },
      }),
    ]);
    const stateCounts = {
      ACTIVE: activeCount,
      ACKNOWLEDGED: acknowledgedCount,
      RESOLVED: resolvedCount,
      MANUAL_RESOLVED: manualResolvedCount,
    };
    return { items, total, page, pageSize, stateCounts, gateways };
  }

  private dateRange(startAt: string, endAt: string): Prisma.DateTimeFilter | undefined {
    const start = startAt ? new Date(startAt) : undefined;
    const end = endAt ? new Date(endAt) : undefined;
    const validStart = start && !Number.isNaN(start.getTime()) ? start : undefined;
    const validEnd = end && !Number.isNaN(end.getTime()) ? end : undefined;
    return validStart || validEnd ? { ...(validStart ? { gte: validStart } : {}), ...(validEnd ? { lte: validEnd } : {}) } : undefined;
  }
}

import { Injectable } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class DeviceLogsService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(actor: AuthUser, page: number, pageSize: number, query: {
    keyword?: string;
    type?: string;
    level?: string;
    source?: string;
    deviceId?: string;
    startAt?: string;
    endAt?: string;
    platformOnly?: boolean;
  }) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    if (actor.role !== 'SUPER_ADMIN' && !actor.tenantId) {
      return { items: [], total: 0, page: safePage, pageSize: safePageSize };
    }
    const keyword = query.keyword?.trim();
    const createdAt = this.dateRange(query.startAt, query.endAt);
    const platformTypes = ['ONLINE', 'OFFLINE', 'MQTT_CONNECTED', 'MQTT_DISCONNECTED'];
    const where: Prisma.DeviceLogWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(query.type ? { type: query.type } : query.platformOnly ? { type: { in: platformTypes } } : {}),
      ...(query.level ? { level: query.level } : {}),
      ...(query.source ? { source: query.source } : {}),
      ...(query.deviceId ? { deviceId: query.deviceId } : {}),
      ...(createdAt ? { createdAt } : {}),
      ...(keyword
        ? {
            OR: [
              { deviceKey: { contains: keyword, mode: 'insensitive' } },
              { deviceName: { contains: keyword, mode: 'insensitive' } },
              { message: { contains: keyword, mode: 'insensitive' } },
            ],
          }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.deviceLog.findMany({
        where,
        include: {
          tenant: { select: { id: true, name: true } },
          device: { select: { id: true, name: true, deviceKey: true, status: true, deviceType: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.deviceLog.count({ where }),
    ]);
    return { items, total, page: safePage, pageSize: safePageSize };
  }

  private dateRange(startAt?: string, endAt?: string): Prisma.DateTimeFilter | undefined {
    const start = startAt ? new Date(startAt) : undefined;
    const end = endAt ? new Date(endAt) : undefined;
    const validStart = start && !Number.isNaN(start.getTime()) ? start : undefined;
    const validEnd = end && !Number.isNaN(end.getTime()) ? end : undefined;
    return validStart || validEnd ? { ...(validStart ? { gte: validStart } : {}), ...(validEnd ? { lte: validEnd } : {}) } : undefined;
  }
}

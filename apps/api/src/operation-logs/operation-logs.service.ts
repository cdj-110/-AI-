import { Injectable } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

export interface OperationLogMeta {
  module: string;
  action: string;
  targetType: string;
  targetId?: string;
  targetName?: string;
  ip?: string;
  userAgent?: string | string[];
  detail?: unknown;
}

@Injectable()
export class OperationLogsService {
  constructor(private readonly prisma: PrismaService) {}

  async record(actor: AuthUser, meta: OperationLogMeta) {
    await this.prisma.operationLog.create({
      data: {
        tenantId: actor.tenantId,
        userId: actor.sub,
        username: actor.username,
        module: meta.module,
        action: meta.action,
        targetType: meta.targetType,
        targetId: meta.targetId,
        targetName: meta.targetName,
        ip: meta.ip,
        userAgent: Array.isArray(meta.userAgent) ? meta.userAgent.join(' ') : meta.userAgent,
        detail: meta.detail === undefined ? undefined : JSON.parse(JSON.stringify(meta.detail)) as Prisma.InputJsonValue,
      },
    });
  }

  async findAll(actor: AuthUser, page: number, pageSize: number, query: { keyword?: string; module?: string; action?: string }) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const keyword = query.keyword?.trim();
    const where: Prisma.OperationLogWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(query.module ? { module: query.module } : {}),
      ...(query.action ? { action: query.action } : {}),
      ...(keyword
        ? {
            OR: [
              { username: { contains: keyword, mode: 'insensitive' } },
              { targetName: { contains: keyword, mode: 'insensitive' } },
              { ip: { contains: keyword, mode: 'insensitive' } },
            ],
          }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.operationLog.findMany({
        where,
        include: {
          tenant: { select: { id: true, name: true } },
          user: { select: { id: true, username: true, nickname: true, role: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.operationLog.count({ where }),
    ]);
    return { items, total, page: safePage, pageSize: safePageSize };
  }
}

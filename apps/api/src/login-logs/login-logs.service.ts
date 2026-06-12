import { Injectable } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class LoginLogsService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(actor: AuthUser, page: number, pageSize: number, keyword = '', success = '') {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const safeSuccess = success === 'true' ? true : success === 'false' ? false : undefined;
    const where: Prisma.LoginLogWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(safeSuccess === undefined ? {} : { success: safeSuccess }),
      ...(keyword
        ? {
            OR: [
              { username: { contains: keyword, mode: 'insensitive' } },
              { ip: { contains: keyword, mode: 'insensitive' } },
              { user: { nickname: { contains: keyword, mode: 'insensitive' } } },
            ],
          }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.loginLog.findMany({
        where,
        include: {
          tenant: { select: { id: true, name: true } },
          user: { select: { id: true, username: true, nickname: true, role: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.loginLog.count({ where }),
    ]);
    return { items, total, page: safePage, pageSize: safePageSize };
  }
}

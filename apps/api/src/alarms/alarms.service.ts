import { ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class AlarmsService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(actor: AuthUser, page: number, pageSize: number, status: string) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.AlarmWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(status ? { status } : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.alarm.findMany({
        where,
        include: {
          device: { select: { id: true, deviceKey: true, name: true } },
          tenant: { select: { id: true, name: true } },
        },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
        orderBy: { createdAt: 'desc' },
      }),
      this.prisma.alarm.count({ where }),
    ]);
    return { items, total, page: safePage, pageSize: safePageSize };
  }

  async resolve(actor: AuthUser, id: string) {
    const alarm = await this.prisma.alarm.findUnique({ where: { id } });
    if (!alarm) throw new NotFoundException('告警不存在');
    if (actor.role !== 'SUPER_ADMIN' && alarm.tenantId !== actor.tenantId) throw new ForbiddenException('无权处理该告警');
    if (alarm.status === 'RESOLVED') return alarm;
    return this.prisma.alarm.update({
      where: { id },
      data: { status: 'RESOLVED', resolvedAt: new Date() },
    });
  }
}

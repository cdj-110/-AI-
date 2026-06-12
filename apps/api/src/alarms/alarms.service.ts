import { ForbiddenException, Injectable, MessageEvent, NotFoundException, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { Pool, PoolClient } from 'pg';
import { filter, map, Observable, Subject } from 'rxjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class AlarmsService implements OnModuleDestroy {
  private readonly postgres = new Pool({
    connectionString: (process.env.DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5432/weikong_iot?schema=public').replace('?schema=public', ''),
  });
  private readonly alarmEvents = new Subject<{ tenantId: string; status: string; [key: string]: unknown }>();
  private listener?: PoolClient;
  private destroying = false;

  constructor(private readonly prisma: PrismaService) {
    void this.listenForAlarmEvents();
  }

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

  stream(actor: AuthUser): Observable<MessageEvent> {
    return this.alarmEvents.pipe(
      filter((alarm) => actor.role === 'SUPER_ADMIN' || alarm.tenantId === actor.tenantId),
      map((alarm) => ({ data: alarm })),
    );
  }

  async onModuleDestroy() {
    this.destroying = true;
    this.listener?.release();
    await this.postgres.end();
  }

  private async listenForAlarmEvents() {
    try {
      this.listener = await this.postgres.connect();
      await this.listener.query('LISTEN alarm_events');
      this.listener.on('notification', (notification) => {
        if (!notification.payload) return;
        try {
          this.alarmEvents.next(JSON.parse(notification.payload));
        } catch {
          console.warn('[api] ignored invalid alarm notification');
        }
      });
      this.listener.on('error', (error) => {
        console.error('[api] alarm listener error', error);
        this.listener?.release();
        this.listener = undefined;
        if (!this.destroying) setTimeout(() => void this.listenForAlarmEvents(), 3000);
      });
    } catch (error) {
      console.error('[api] failed to listen for alarm events', error);
      if (!this.destroying) setTimeout(() => void this.listenForAlarmEvents(), 3000);
    }
  }
}

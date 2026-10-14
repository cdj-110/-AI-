import { ForbiddenException, Injectable, MessageEvent, NotFoundException, OnModuleDestroy } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { Pool, PoolClient } from 'pg';
import { filter, map, Observable, Subject } from 'rxjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { deviceAccessWhere } from '../common/user-permissions';

@Injectable()
export class AlarmsService implements OnModuleDestroy {
  private readonly postgres = new Pool({
    connectionString: (process.env.DATABASE_URL ?? 'postgresql://weikong@localhost:5432/weikong_iot?schema=public').replace('?schema=public', ''),
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
      ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM' ? { device: deviceAccessWhere(actor) } : {}),
      ...(status ? { status } : {}),
    };
    const user = await this.prisma.user.findUnique({
      where: { id: actor.sub },
      select: { alarmNotificationsReadAt: true },
    });
    const unreadWhere: Prisma.AlarmWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM' ? { device: deviceAccessWhere(actor) } : {}),
      ...(user?.alarmNotificationsReadAt ? { createdAt: { gt: user.alarmNotificationsReadAt } } : {}),
    };
    const [items, total, unreadTotal] = await this.prisma.$transaction([
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
      this.prisma.alarm.count({ where: unreadWhere }),
    ]);
    return {
      items: items.map((item) => ({
        ...item,
        notificationRead: Boolean(user?.alarmNotificationsReadAt && item.createdAt <= user.alarmNotificationsReadAt),
      })),
      total,
      unreadTotal,
      page: safePage,
      pageSize: safePageSize,
    };
  }

  async markNotificationsRead(actor: AuthUser) {
    const readAt = new Date();
    await this.prisma.user.update({
      where: { id: actor.sub },
      data: { alarmNotificationsReadAt: readAt },
    });
    return { readAt, unreadTotal: 0 };
  }

  async resolve(actor: AuthUser, id: string) {
    const alarm = await this.prisma.alarm.findUnique({
      where: { id },
      include: { device: { select: { name: true } } },
    });
    if (!alarm) throw new NotFoundException('告警不存在');
    if (actor.role !== 'SUPER_ADMIN' && alarm.tenantId !== actor.tenantId) throw new ForbiddenException('无权处理该告警');
    if (actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM') {
      const accessible = await this.prisma.device.count({ where: { ...deviceAccessWhere(actor), id: alarm.deviceId } });
      if (!accessible) throw new ForbiddenException('无权处理该设备的告警');
    }
    if (alarm.status === 'RESOLVED') return alarm;
    const resolved = await this.prisma.alarm.update({
      where: { id },
      data: { status: 'RESOLVED', resolvedAt: new Date() },
      include: { device: { select: { name: true } } },
    });
    await this.notifyAlarmEvent({
      id: resolved.id,
      tenantId: resolved.tenantId,
      deviceId: resolved.deviceId,
      deviceName: resolved.device.name,
      type: resolved.type,
      level: resolved.level,
      message: resolved.message,
      status: resolved.status,
      createdAt: resolved.createdAt,
      resolvedAt: resolved.resolvedAt,
    });
    return resolved;
  }

  stream(actor: AuthUser): Observable<MessageEvent> {
    if (actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM') {
      return new Observable<MessageEvent>((subscriber) => {
        let liveSubscription: { unsubscribe: () => void } | undefined;
        let refreshTimer: ReturnType<typeof setInterval> | undefined;
        let accessibleIds = new Set<string>();
        let refreshing = false;
        let closed = false;
        const refreshAccess = async () => {
          if (refreshing || closed) return;
          refreshing = true;
          try {
            const devices = await this.prisma.device.findMany({ where: deviceAccessWhere(actor), select: { id: true } });
            accessibleIds = new Set(devices.map(device => device.id));
          } finally {
            refreshing = false;
          }
        };
        void refreshAccess()
          .then(() => {
            liveSubscription = this.alarmEvents.pipe(
              filter(alarm => typeof alarm.deviceId === 'string' && accessibleIds.has(alarm.deviceId)),
              map(alarm => ({ data: alarm })),
            ).subscribe(subscriber);
            refreshTimer = setInterval(() => { void refreshAccess().catch(error => subscriber.error(error)); }, 3000);
          })
          .catch(error => subscriber.error(error));
        return () => {
          closed = true;
          if (refreshTimer) clearInterval(refreshTimer);
          liveSubscription?.unsubscribe();
        };
      });
    }
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

  private async notifyAlarmEvent(alarm: Record<string, unknown>) {
    await this.prisma.$executeRawUnsafe(
      'SELECT pg_notify($1, $2)',
      'alarm_events',
      JSON.stringify(alarm),
    );
  }
}

import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import { historyRetention } from './history-retention';

@Injectable()
export class SubscriptionLifecycleService implements OnModuleInit, OnModuleDestroy {
  private timer?: ReturnType<typeof setInterval>;
  constructor(private readonly prisma: PrismaService) {}

  onModuleInit() {
    void this.run();
    this.timer = setInterval(() => void this.run(), 60 * 60 * 1000);
    this.timer.unref?.();
  }

  onModuleDestroy() { if (this.timer) clearInterval(this.timer); }

  async run(tenantId?: string) {
    const now = new Date();
    const subscriptions = await this.prisma.tenantSubscription.findMany({
      where: { ...(tenantId ? { tenantId } : {}) },
      include: { plan: true },
    });
    for (const subscription of subscriptions) {
      const retention = historyRetention(subscription.plan, subscription, now);
      const losses = [subscription.expiresAt, retention.addonExpiresAt].filter((v): v is Date => Boolean(v));
      for (const lossAt of losses) {
        const days = Math.ceil((lossAt.getTime() - now.getTime()) / 86_400_000);
        const key = `${subscription.id}:HISTORY:${lossAt.toISOString()}`;
        if (days > 0) {
          const threshold = [1, 3, 7, 30].find(v => days <= v);
          if (threshold) await this.notify(subscription.tenantId, `${key}:WARN:${threshold}`, 'HISTORY_EXPIRING', '历史存储权益即将到期', `历史存储权益将于${this.date(lossAt)}到期；扩展包不会随套餐续费自动续购。到期后的清理宽限期为7天，请及时续订或导出。`);
        } else if (days > -7) {
          await this.notify(subscription.tenantId, `${key}:EXPIRED`, 'HISTORY_EXPIRED', '历史存储权益到期，清理宽限期7天', `历史查询将恢复为${retention.label}；超期数据临时可查询和导出至${this.date(new Date(lossAt.getTime() + 7 * 86_400_000))}，已清理的数据无法恢复。`);
        } else if (days >= -30) {
          await this.notify(subscription.tenantId, `${key}:CLEANUP`, 'HISTORY_CLEANUP', '历史数据清理宽限期已结束', '超出当前保存期限的数据将进入清理范围。实际清理以平台维护任务为准；购买扩展包不能恢复已清理的数据。');
        }
      }
      if (subscription.historyGraceEndsAt && subscription.historyGraceEndsAt <= now) await this.notify(subscription.tenantId, `${subscription.id}:HISTORY_PLAN_CLEANUP:${subscription.historyGraceEndsAt.toISOString()}`, 'HISTORY_CLEANUP', '历史数据清理宽限期已结束', '套餐保存期限调整的7天清理宽限期已结束，超期数据将进入清理范围。');
      if (subscription.status === 'SUSPENDED' || subscription.status === 'EXPIRED') continue;
      let nextStatus = subscription.status;
      if (subscription.graceEndsAt <= now) nextStatus = 'EXPIRED';
      else if (subscription.expiresAt <= now) nextStatus = 'GRACE';
      if (nextStatus !== subscription.status) {
        await this.prisma.tenantSubscription.update({ where: { id: subscription.id }, data: { status: nextStatus } });
        await this.notify(
          subscription.tenantId,
          `${subscription.id}:${nextStatus}`,
          nextStatus === 'GRACE' ? 'SUBSCRIPTION_GRACE' : 'SUBSCRIPTION_EXPIRED',
          nextStatus === 'GRACE' ? '版本已到期，已进入宽限期' : '版本权益已到期',
          nextStatus === 'GRACE'
            ? `“${subscription.plan.name}”已到期，请在 ${this.date(subscription.graceEndsAt)} 前完成续费。`
            : `“${subscription.plan.name}”宽限期已结束，新增设备、点位、组态和用户功能已暂停。`,
        );
      }
      if (nextStatus === 'ACTIVE') {
        const days = Math.ceil((subscription.expiresAt.getTime() - now.getTime()) / 86_400_000);
        const threshold = [1, 3, 7, 30].find(value => days <= value);
        if (threshold) {
          await this.notify(subscription.tenantId, `${subscription.id}:EXPIRING:${threshold}`, 'SUBSCRIPTION_EXPIRING', `版本将在 ${days} 天后到期`, `“${subscription.plan.name}”将于 ${this.date(subscription.expiresAt)} 到期，请及时续费。`);
        }
      }
    }
  }

  private async notify(tenantId: string, dedupeKey: string, type: string, title: string, message: string) {
    await this.prisma.subscriptionNotification.create({ data: { tenantId, dedupeKey, type, title, message } }).catch(error => {
      if (error?.code !== 'P2002') throw error;
    });
  }

  private date(value: Date) { return value.toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' }); }
}

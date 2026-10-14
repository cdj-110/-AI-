import { ForbiddenException, Injectable } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import { historyRetention } from './history-retention';

@Injectable()
export class SubscriptionLimitsService {
  constructor(private readonly prisma: PrismaService) {}

  async history(tenantId: string, now = new Date()) {
    const c = await this.context(tenantId);
    return historyRetention(c.plan, c.subscription, now);
  }

  async assertCanAddDevices(tenantId: string, increment = 1) { const c = await this.context(tenantId); this.assertActive(c); this.assertAvailable('设备', c.planName, await this.prisma.device.count({ where: { tenantId } }), c.limits.devices, increment); }
  async assertCanAddPoints(tenantId: string, increment = 1) { const c = await this.context(tenantId); this.assertActive(c); this.assertAvailable('云点位', c.planName, await this.prisma.deviceMetric.count({ where: { device: { tenantId }, enabled: true, ignored: false } }), c.limits.points, increment); }
  async assertCanAddScadaProjects(tenantId: string, increment = 1) { const c = await this.context(tenantId); this.assertActive(c); this.assertAvailable('云组态项目', c.planName, await this.prisma.scadaProject.count({ where: { tenantId } }), c.limits.scadaProjects, increment); }
  async assertCanAddUsers(tenantId: string, increment = 1) { const c = await this.context(tenantId); this.assertActive(c); this.assertAvailable('用户', c.planName, await this.prisma.user.count({ where: { tenantId } }), c.limits.users, increment); }

  async usage(tenantId: string) {
    const context = await this.context(tenantId);
    const [devices, points, scadaProjects, users] = await this.prisma.$transaction([
      this.prisma.device.count({ where: { tenantId } }),
      this.prisma.deviceMetric.count({ where: { device: { tenantId }, enabled: true, ignored: false } }),
      this.prisma.scadaProject.count({ where: { tenantId } }),
      this.prisma.user.count({ where: { tenantId } }),
    ]);
    return {
      plan: context.plan,
      subscription: context.subscription,
      historyRetention: historyRetention(context.plan, context.subscription),
      usage: {
        devices: { used: devices, limit: context.limits.devices },
        points: { used: points, limit: context.limits.points },
        scadaProjects: { used: scadaProjects, limit: context.limits.scadaProjects },
        users: { used: users, limit: context.limits.users },
      },
    };
  }

  private async context(tenantId: string) {
    const tenant = await this.prisma.tenant.findUnique({
      where: { id: tenantId },
      include: { subscriptionPlan: true, subscription: { include: { plan: true } } },
    });
    if (!tenant) throw new ForbiddenException('当前租户不存在');
    const subscription = tenant.subscription;
    const plan = subscription?.plan ?? tenant.subscriptionPlan;
    const effective = (base: number, addon: number) => base === 0 ? 0 : base + addon;
    return {
      plan,
      subscription,
      planName: plan?.name ?? '未配置版本',
      status: subscription?.status ?? 'ACTIVE',
      limits: subscription ? {
        devices: effective(subscription.baseDeviceLimit, subscription.addonDeviceLimit),
        points: effective(subscription.basePointLimit, subscription.addonPointLimit),
        scadaProjects: effective(subscription.baseScadaProjectLimit, subscription.addonScadaProjectLimit),
        users: effective(subscription.baseUserLimit, subscription.addonUserLimit),
      } : {
        devices: plan?.deviceLimit ?? 0,
        points: plan?.pointLimit ?? 0,
        scadaProjects: plan?.scadaProjectLimit ?? 0,
        users: plan?.userLimit ?? 0,
      },
    };
  }

  private assertActive(context: { status: string; planName: string }) {
    if (context.status === 'EXPIRED' || context.status === 'SUSPENDED') throw new ForbiddenException(`“${context.planName}”权益已到期，请续费后再操作`);
  }

  private assertAvailable(resource: string, planName: string, used: number, limit: number, increment: number) {
    if (limit > 0 && used + increment > limit) throw new ForbiddenException(`当前“${planName}”${resource}额度为 ${limit}，已使用 ${used}，请升级版本或购买扩容包`);
  }
}

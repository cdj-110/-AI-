import { BadRequestException, ConflictException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { randomBytes, randomUUID } from 'node:crypto';
import { promises as fs } from 'node:fs';
import { extname, join, resolve } from 'node:path';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { PrismaService } from '../prisma/prisma.service';
import { CreateSubscriptionPlanDto, ReviewSubscriptionOrderDto, SaveQuotaAddonDto, SubmitPlanUpgradeDto, SubmitSubscriptionOrderDto, UpdateSubscriptionPlanDto, UpgradeRequestQueryDto } from './dto/subscription.dto';
import { SubscriptionLifecycleService } from './subscription-lifecycle.service';
import { SubscriptionLimitsService } from './subscription-limits.service';
import { AlipayPaymentService, AlipayTradeResult } from './alipay-payment.service';
import { historyRetention, protectHistoryReduction, shiftHistoryMonths } from './history-retention';

interface VoucherFile { buffer: Buffer; mimetype: string; originalname: string; size: number }

@Injectable()
export class SubscriptionsService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly limits: SubscriptionLimitsService,
    private readonly lifecycle: SubscriptionLifecycleService,
    private readonly logs: OperationLogsService,
    private readonly alipay: AlipayPaymentService,
  ) {}

  listPlans(actor: AuthUser) {
    return this.prisma.subscriptionPlan.findMany({
      where: actor.role === 'SUPER_ADMIN' ? {} : { enabled: true },
      include: { _count: { select: { tenants: true, subscriptions: true, orders: true } } },
      orderBy: [{ sortOrder: 'asc' }, { priceCents: 'asc' }, { createdAt: 'asc' }],
    });
  }

  async createPlan(actor: AuthUser, dto: CreateSubscriptionPlanDto) {
    this.validateHistoryPlan(dto.historyRetentionDays ?? 7, dto.historyRetentionMonths ?? 0);
    const plan = await this.prisma.subscriptionPlan.create({ data: this.cleanPlan(dto) });
    await this.logs.record(actor, { module: '套餐管理', action: '新增套餐', targetType: 'SubscriptionPlan', targetId: plan.id, targetName: plan.name, detail: plan });
    return plan;
  }

  async updatePlan(actor: AuthUser, id: string, dto: UpdateSubscriptionPlanDto) {
    const previous = await this.findPlan(id);
    this.validateHistoryPlan(dto.historyRetentionDays ?? previous.historyRetentionDays, dto.historyRetentionMonths ?? previous.historyRetentionMonths);
    const plan = await this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT "id" FROM "SubscriptionPlan" WHERE "id" = ${id} FOR UPDATE`;
      const old = await tx.subscriptionPlan.findUniqueOrThrow({ where: { id } });
      this.validateHistoryPlan(dto.historyRetentionDays ?? old.historyRetentionDays, dto.historyRetentionMonths ?? old.historyRetentionMonths);
      const now = new Date();
      const planProtection = protectHistoryReduction(historyRetention(old, null, now), historyRetention({ ...old, ...dto }, null, now), now);
      const updated = await tx.subscriptionPlan.update({ where: { id }, data: { ...this.cleanPlan(dto), ...planProtection } });
      const subscriptions = await tx.tenantSubscription.findMany({ where: { planId: id } });
      if (planProtection.historyGraceEndsAt) {
        const tenants = await tx.tenant.findMany({ where: { subscriptionPlanId: id, subscription: null }, select: { id: true } });
        for (const tenant of tenants) await tx.subscriptionNotification.create({ data: { tenantId: tenant.id, dedupeKey: `HISTORY_PLAN:${tenant.id}:${now.toISOString()}`, type: 'HISTORY_REDUCED', title: '历史保存期限已缩短，清理宽限期7天', message: `历史期限已调整，请在${planProtection.historyGraceEndsAt.toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' })}前导出需要保留的数据。已清理的数据无法恢复。` } });
      }
      for (const sub of subscriptions) {
        const protection = protectHistoryReduction(historyRetention(old, sub, now), historyRetention(updated, sub, now), now);
        if (!protection.historyGraceEndsAt) continue;
        await tx.tenantSubscription.update({ where: { id: sub.id }, data: protection });
        await tx.subscriptionNotification.create({ data: { tenantId: sub.tenantId, dedupeKey: `HISTORY_PLAN:${sub.id}:${now.toISOString()}`, type: 'HISTORY_REDUCED', title: '历史保存期限已缩短，清理宽限期7天', message: `套餐历史期限已调整为${historyRetention(updated, sub, now).label}。请在${protection.historyGraceEndsAt.toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' })}前导出需要保留的数据；已清理的数据无法恢复。` } });
      }
      return updated;
    });
    await this.logs.record(actor, { module: '套餐管理', action: '修改套餐', targetType: 'SubscriptionPlan', targetId: plan.id, targetName: plan.name, detail: plan });
    return plan;
  }

  listAddons(actor: AuthUser) {
    return this.prisma.quotaAddon.findMany({ where: actor.role === 'SUPER_ADMIN' ? {} : { enabled: true }, orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] });
  }

  async adminNotifications(actor: AuthUser) {
    await this.lifecycle.run();
    const [user, orders, subscriptionNotices] = await Promise.all([
      this.prisma.user.findUnique({ where: { id: actor.sub }, select: { adminNotificationsReadAt: true } }),
      this.prisma.subscriptionOrder.findMany({
        where: { status: 'PENDING_REVIEW' },
        include: { tenant: { select: { name: true } }, targetPlan: { select: { name: true } }, addon: { select: { name: true } } },
        orderBy: { requestedAt: 'desc' }, take: 30,
      }),
      this.prisma.subscriptionNotification.findMany({
        where: { type: { in: ['SUBSCRIPTION_EXPIRING', 'SUBSCRIPTION_GRACE', 'SUBSCRIPTION_EXPIRED', 'HISTORY_EXPIRING', 'HISTORY_REDUCED', 'HISTORY_EXPIRED', 'HISTORY_CLEANUP'] } },
        include: { tenant: { select: { name: true } } }, orderBy: { createdAt: 'desc' }, take: 30,
      }),
    ]);
    const readAt = user?.adminNotificationsReadAt;
    const orderItems = orders.map(order => ({
      id: `ORDER:${order.id}`, category: 'ORDER', title: `待审核：${this.orderTypeName(order.orderType)}订单`,
      message: `${order.tenant.name}提交了${order.targetPlan?.name ?? order.addon?.name ?? '订阅'}订单，金额 ¥${(order.amountCents / 100).toFixed(2)}。`,
      createdAt: order.requestedAt, route: '/finance-reviews', read: Boolean(readAt && order.requestedAt <= readAt),
    }));
    const subscriptionItems = subscriptionNotices.map(notice => ({
      id: `SUBSCRIPTION:${notice.id}`, category: 'SUBSCRIPTION', title: `${notice.tenant.name}：${notice.title}`,
      message: notice.message, createdAt: notice.createdAt, route: '/customer-subscriptions', read: Boolean(readAt && notice.createdAt <= readAt),
    }));
    const items = [...orderItems, ...subscriptionItems].sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime()).slice(0, 50);
    return { items, unreadTotal: items.filter(item => !item.read).length };
  }

  async markAdminNotificationsRead(actor: AuthUser) {
    const readAt = new Date();
    await this.prisma.user.update({ where: { id: actor.sub }, data: { adminNotificationsReadAt: readAt } });
    return { readAt, unreadTotal: 0 };
  }

  async customerSubscriptions() {
    await this.lifecycle.run();
    const subscriptions = await this.prisma.tenantSubscription.findMany({
      include: {
        tenant: { select: { id: true, name: true, _count: { select: { devices: true, users: true, scadaProjects: true } } } },
        plan: true,
        orders: { orderBy: { requestedAt: 'desc' }, take: 1, select: { orderNo: true, status: true, orderType: true, requestedAt: true } },
      },
      orderBy: [{ status: 'asc' }, { expiresAt: 'asc' }],
    });
    return Promise.all(subscriptions.map(async subscription => ({
      ...subscription,
      usage: {
        devices: subscription.tenant._count.devices,
        users: subscription.tenant._count.users,
        scadaProjects: subscription.tenant._count.scadaProjects,
        points: await this.prisma.deviceMetric.count({ where: { device: { tenantId: subscription.tenantId }, enabled: true, ignored: false } }),
      },
    })));
  }

  async createAddon(actor: AuthUser, dto: SaveQuotaAddonDto) {
    const addon = await this.prisma.quotaAddon.create({ data: this.cleanAddon(dto) });
    await this.logs.record(actor, { module: '扩容包管理', action: '新增扩容包', targetType: 'QuotaAddon', targetId: addon.id, targetName: addon.name, detail: addon });
    return addon;
  }

  async updateAddon(actor: AuthUser, id: string, dto: SaveQuotaAddonDto) {
    const exists = await this.prisma.quotaAddon.findUnique({ where: { id } });
    if (!exists) throw new NotFoundException('扩容包不存在');
    const addon = await this.prisma.quotaAddon.update({ where: { id }, data: this.cleanAddon(dto) });
    await this.logs.record(actor, { module: '扩容包管理', action: '修改扩容包', targetType: 'QuotaAddon', targetId: addon.id, targetName: addon.name, detail: addon });
    return addon;
  }

  async current(actor: AuthUser) {
    const tenantId = this.tenantId(actor);
    await this.lifecycle.run(tenantId);
    const [summary, pendingOrder, notifications, entitlementChanges] = await Promise.all([
      this.limits.usage(tenantId),
      this.prisma.subscriptionOrder.findFirst({ where: { tenantId, status: { in: ['PENDING_PAYMENT', 'PENDING_REVIEW'] } }, include: this.orderInclude, orderBy: { requestedAt: 'desc' } }),
      this.prisma.subscriptionNotification.findMany({ where: { tenantId }, orderBy: { createdAt: 'desc' }, take: 20 }),
      this.prisma.entitlementChange.findMany({ where: { tenantId }, orderBy: { createdAt: 'desc' }, take: 20 }),
    ]);
    return { ...summary, pendingOrder, pendingRequest: pendingOrder, notifications, entitlementChanges };
  }

  myRequests(actor: AuthUser) {
    return this.prisma.subscriptionOrder.findMany({ where: { tenantId: this.tenantId(actor) }, include: this.orderInclude, orderBy: { requestedAt: 'desc' } });
  }

  submit(actor: AuthUser, dto: SubmitPlanUpgradeDto, file?: VoucherFile) {
    return this.submitOrder(actor, { orderType: 'UPGRADE', targetPlanId: dto.targetPlanId, billingCycle: dto.billingCycle ?? 'YEARLY' }, file);
  }

  async submitOrder(actor: AuthUser, dto: SubmitSubscriptionOrderDto, file?: VoucherFile) {
    return this.submitOrderForTenant(actor, this.tenantId(actor), dto, file);
  }

  async createAlipayOrder(actor: AuthUser, dto: SubmitSubscriptionOrderDto, origin?: string) {
    if (!this.alipay.available()) throw new BadRequestException('支付宝沙箱支付尚未完成配置');
    const order = await this.submitOrderForTenant(actor, this.tenantId(actor), { ...dto, paymentMethod: 'ALIPAY' });
    return { order, checkout: this.checkout(order, origin) };
  }

  async alipayCheckout(actor: AuthUser, id: string, origin?: string) {
    const order = await this.prisma.subscriptionOrder.findFirst({ where: { id, tenantId: this.tenantId(actor), paymentMethod: 'ALIPAY' }, include: this.orderInclude });
    if (!order) throw new NotFoundException('支付宝订单不存在');
    if (order.status !== 'PENDING_PAYMENT') throw new ConflictException('该订单已完成或已关闭，不能继续支付');
    return { order, checkout: this.checkout(order, origin) };
  }

  async submitOrderForTenant(actor: AuthUser, tenantId: string, dto: SubmitSubscriptionOrderDto, file?: VoucherFile) {
    const [tenant, subscription, pending] = await Promise.all([
      this.prisma.tenant.findUnique({ where: { id: tenantId } }),
      this.prisma.tenantSubscription.findUnique({ where: { tenantId }, include: { plan: true } }),
      this.prisma.subscriptionOrder.findFirst({ where: { tenantId, status: { in: ['PENDING_PAYMENT', 'PENDING_REVIEW'] } } }),
    ]);
    if (!tenant) throw new NotFoundException('租户不存在');
    if (pending) throw new ConflictException(`订单 ${pending.orderNo} ${pending.status === 'PENDING_PAYMENT' ? '正在等待支付' : '正在等待财务审核'}，请勿重复提交`);
    let billingCycle = dto.orderType === 'EXPANSION' ? 'YEARLY' : (dto.billingCycle ?? subscription?.billingCycle ?? 'YEARLY');
    let targetPlan = null;
    let addon = null;
    let amountCents = 0;
    let durationMonths = billingCycle === 'MONTHLY' ? 1 : 12;
    let addonQuantity = 1;

    if (dto.orderType === 'UPGRADE') {
      if (!dto.targetPlanId) throw new BadRequestException('请选择目标套餐');
      targetPlan = await this.prisma.subscriptionPlan.findUnique({ where: { id: dto.targetPlanId } });
      if (!targetPlan || !targetPlan.enabled) throw new NotFoundException('目标套餐不存在或已停用');
      if (targetPlan.quoteOnly) throw new BadRequestException('该套餐需要联系平台商务报价，不能直接提交付款订单');
      if (!targetPlan.billingCycles.includes(billingCycle)) throw new BadRequestException(`该套餐不支持${billingCycle === 'MONTHLY' ? '月付' : '年付'}`);
      if (subscription?.planId === targetPlan.id) throw new BadRequestException('当前已是该套餐，请选择续费');
      if (subscription && targetPlan.sortOrder <= subscription.plan.sortOrder) throw new BadRequestException('请选择高于当前版本的套餐');
      amountCents = billingCycle === 'MONTHLY' ? targetPlan.monthlyPriceCents : targetPlan.priceCents;
    } else if (dto.orderType === 'RENEW') {
      if (!subscription) throw new BadRequestException('当前没有可续费的订阅');
      targetPlan = subscription.plan;
      if (targetPlan.quoteOnly) throw new BadRequestException('该套餐需要联系平台商务报价，不能直接提交付款订单');
      if (!targetPlan.billingCycles.includes(billingCycle)) billingCycle = targetPlan.billingCycles[0] ?? 'YEARLY';
      amountCents = billingCycle === 'MONTHLY' ? targetPlan.monthlyPriceCents : targetPlan.priceCents;
    } else {
      if (!subscription || subscription.status === 'EXPIRED') throw new BadRequestException('请先续费有效订阅，再购买扩容包');
      if (!dto.addonId) throw new BadRequestException('请选择扩容包');
      addon = await this.prisma.quotaAddon.findUnique({ where: { id: dto.addonId } });
      if (!addon || !addon.enabled) throw new NotFoundException('扩容包不存在或已停用');
      if (addon.pricePending) throw new BadRequestException('该扩容包价格待定，暂不开放购买');
      if (addon.resourceType === 'HISTORY' && (addon.priceCents <= 0 || subscription.expiresAt <= new Date())) throw new BadRequestException('历史扩展包需已定价且订阅未到期');
      if (addon.quoteOnly) throw new BadRequestException('该扩容包需要联系平台商务报价，不能直接提交付款订单');
      const baseLimit = { DEVICE: subscription.baseDeviceLimit, POINT: subscription.basePointLimit, SCADA: subscription.baseScadaProjectLimit, USER: subscription.baseUserLimit }[addon.resourceType];
      if (baseLimit === 0) throw new BadRequestException(`当前套餐的${({ DEVICE: '设备', POINT: '点位', SCADA: '组态项目', USER: '用户' } as Record<string,string>)[addon.resourceType]}额度已不限量，无需购买扩容包`);
      addonQuantity = dto.addonQuantity ?? 1;
      amountCents = addon.priceCents * addonQuantity;
      durationMonths = 0;
    }

    const paymentMethod = dto.paymentMethod === 'ALIPAY' ? 'ALIPAY' : 'MANUAL';
    if (paymentMethod === 'MANUAL' && amountCents > 0 && !file) throw new BadRequestException('请上传付款凭证');
    if (paymentMethod === 'ALIPAY' && file) throw new BadRequestException('支付宝订单不需要上传付款凭证');
    if (file) this.validateVoucher(file);
    const storedName = file ? `${tenantId}-${Date.now()}-${randomUUID()}${this.safeExtension(file.mimetype, file.originalname)}` : '';
    if (file) { await fs.mkdir(this.voucherDirectory(), { recursive: true }); await fs.writeFile(join(this.voucherDirectory(), storedName), file.buffer, { flag: 'wx' }); }
    try {
      const order = await this.prisma.subscriptionOrder.create({
        data: {
          orderNo: this.orderNo(), tenantId, subscriptionId: subscription?.id, orderType: dto.orderType,
          targetPlanId: targetPlan?.id, addonId: addon?.id, addonQuantity, billingCycle, durationMonths,
          historyAddonMonths: addon?.resourceType === 'HISTORY' ? 3 * addonQuantity : 0,
          amountCents, paymentAmountCents: paymentMethod === 'ALIPAY' ? this.alipay.paymentAmount(amountCents) : null,
          currency: targetPlan?.currency ?? addon?.currency ?? 'CNY', status: paymentMethod === 'ALIPAY' ? 'PENDING_PAYMENT' : 'PENDING_REVIEW',
          paymentMethod, paymentStatus: paymentMethod === 'ALIPAY' ? 'WAIT_BUYER_PAY' : 'UNPAID',
          requestedById: actor.sub, requestedByName: actor.username,
          ...(file ? { paymentProofs: { create: { voucherFileName: storedName, voucherOriginalName: file.originalname.slice(0, 255), voucherMimeType: file.mimetype, paymentReference: dto.paymentReference?.trim() || null, paidBy: dto.paidBy?.trim() || null, paidAt: dto.paidAt ? new Date(dto.paidAt) : null } } } : {}),
        },
        include: this.orderInclude,
      });
      await this.logs.record(actor, { module: '订阅订单', action: `提交${this.orderTypeName(dto.orderType)}订单`, targetType: 'SubscriptionOrder', targetId: order.id, targetName: order.orderNo, detail: { amountCents, paymentMethod, paymentAmountCents: order.paymentAmountCents, targetPlanId: targetPlan?.id, addonId: addon?.id } });
      return order;
    } catch (error) {
      if (file) await fs.unlink(join(this.voucherDirectory(), storedName)).catch(() => undefined);
      throw error;
    }
  }

  async alipayPaymentStatus(actor: AuthUser, id: string) {
    const tenantId = this.tenantId(actor);
    let order = await this.prisma.subscriptionOrder.findFirst({ where: { id, tenantId, paymentMethod: 'ALIPAY' }, include: this.orderInclude });
    if (!order) throw new NotFoundException('支付宝订单不存在');
    if (order.status === 'PENDING_PAYMENT') {
      const result = await this.alipay.query(order.orderNo);
      order = await this.applyAlipayResult(order.orderNo, result);
    }
    return order;
  }

  async processAlipayNotification(payload: Record<string, unknown>) {
    if (!this.alipay.verifyNotification(payload)) throw new BadRequestException('支付宝回调验签失败');
    this.alipay.validateNotificationIdentity(payload);
    const result: AlipayTradeResult = {
      out_trade_no: String(payload.out_trade_no ?? ''), trade_no: String(payload.trade_no ?? ''),
      trade_status: String(payload.trade_status ?? ''), total_amount: String(payload.total_amount ?? ''), code: '10000',
    };
    if (!result.out_trade_no) throw new BadRequestException('支付宝回调缺少商户订单号');
    await this.applyAlipayResult(result.out_trade_no, result);
    return true;
  }

  async listRequests(query: UpgradeRequestQueryDto) {
    const keyword = query.keyword?.trim();
    const where: Prisma.SubscriptionOrderWhereInput = {
      ...(query.status ? { status: query.status } : {}), ...(query.orderType ? { orderType: query.orderType } : {}),
      ...(keyword ? { OR: [{ orderNo: { contains: keyword, mode: 'insensitive' } }, { tenant: { name: { contains: keyword, mode: 'insensitive' } } }, { requestedByName: { contains: keyword, mode: 'insensitive' } }, { targetPlan: { name: { contains: keyword, mode: 'insensitive' } } }] } : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.subscriptionOrder.findMany({ where, include: this.orderInclude, skip: (query.page - 1) * query.pageSize, take: query.pageSize, orderBy: { requestedAt: 'desc' } }),
      this.prisma.subscriptionOrder.count({ where }),
    ]);
    return { items, total, page: query.page, pageSize: query.pageSize };
  }

  async review(actor: AuthUser, id: string, dto: ReviewSubscriptionOrderDto) {
    const order = await this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT "id" FROM "SubscriptionOrder" WHERE "id" = ${id} FOR UPDATE`;
      const current = await tx.subscriptionOrder.findUnique({ where: { id }, include: { targetPlan: true, addon: true, subscription: true, tenant: true } });
      if (!current) throw new NotFoundException('订单不存在');
      if (current.status !== 'PENDING_REVIEW') throw new ConflictException('该订单已经完成审核，不能重复操作');
      const now = new Date();
      if (dto.decision === 'APPROVED') await this.activateOrder(tx, current, now);
      await tx.financeReview.create({ data: { orderId: current.id, decision: dto.decision, reviewerId: actor.sub, reviewerName: actor.username, note: dto.note?.trim() || null } });
      await tx.subscriptionNotification.create({ data: {
        tenantId: current.tenantId, dedupeKey: `ORDER:${current.id}:${dto.decision}`,
        type: dto.decision === 'APPROVED' ? 'ORDER_APPROVED' : 'ORDER_REJECTED',
        title: dto.decision === 'APPROVED' ? '订单审核通过，权益已生效' : '订单审核未通过',
        message: dto.decision === 'APPROVED' ? `订单 ${current.orderNo} 已完成开通。` : `订单 ${current.orderNo} 已驳回：${dto.note?.trim() || '请联系平台财务人员'}`,
      } });
      return tx.subscriptionOrder.update({ where: { id }, data: { status: dto.decision, activatedAt: dto.decision === 'APPROVED' ? now : null }, include: this.orderInclude });
    });
    await this.logs.record(actor, { module: '财务审核', action: dto.decision === 'APPROVED' ? '审核通过并开通权益' : '审核驳回', targetType: 'SubscriptionOrder', targetId: order.id, targetName: order.orderNo, detail: { note: dto.note, orderType: order.orderType } });
    return order;
  }

  async voucher(orderId: string) {
    const proof = await this.prisma.paymentProof.findFirst({ where: { orderId }, orderBy: { createdAt: 'desc' } });
    if (!proof) throw new NotFoundException('订单没有付款凭证');
    const absolutePath = resolve(this.voucherDirectory(), proof.voucherFileName);
    if (!absolutePath.startsWith(`${this.voucherDirectory()}${process.platform === 'win32' ? '\\' : '/'}`)) throw new BadRequestException('付款凭证路径无效');
    await fs.access(absolutePath).catch(() => { throw new NotFoundException('付款凭证文件不存在'); });
    return { ...proof, absolutePath };
  }

  async markNotificationsRead(actor: AuthUser) {
    await this.prisma.subscriptionNotification.updateMany({ where: { tenantId: this.tenantId(actor), readAt: null }, data: { readAt: new Date() } });
    return { success: true };
  }

  async notifications(actor: AuthUser) {
    const tenantId = this.tenantId(actor);
    await this.lifecycle.run(tenantId);
    const [items, unreadTotal] = await this.prisma.$transaction([
      this.prisma.subscriptionNotification.findMany({ where: { tenantId }, orderBy: { createdAt: 'desc' }, take: 30 }),
      this.prisma.subscriptionNotification.count({ where: { tenantId, readAt: null } }),
    ]);
    return { items, unreadTotal };
  }

  private async activateOrder(tx: Prisma.TransactionClient, order: any, now: Date) {
    await tx.$queryRaw`SELECT "id" FROM "Tenant" WHERE "id" = ${order.tenantId} FOR UPDATE`;
    let subscription = await tx.tenantSubscription.findUnique({ where: { tenantId: order.tenantId } });
    if (order.orderType === 'UPGRADE') {
      if (!order.targetPlan) throw new BadRequestException('订单目标套餐不存在');
      const expiresAt = this.addMonths(now, order.durationMonths);
      const data = {
        planId: order.targetPlan.id, status: 'ACTIVE', billingCycle: order.billingCycle, startedAt: now, expiresAt,
        graceEndsAt: this.addDays(expiresAt, order.targetPlan.gracePeriodDays), baseDeviceLimit: order.targetPlan.deviceLimit,
        basePointLimit: order.targetPlan.pointLimit, baseScadaProjectLimit: order.targetPlan.scadaProjectLimit, baseUserLimit: order.targetPlan.userLimit,
        ...(subscription ? protectHistoryReduction(historyRetention(await tx.subscriptionPlan.findUnique({ where: { id: subscription.planId } }), subscription, now), historyRetention(order.targetPlan, { ...subscription, expiresAt }, now), now) : {}),
      };
      subscription = subscription
        ? await tx.tenantSubscription.update({ where: { id: subscription.id }, data })
        : await tx.tenantSubscription.create({ data: { tenantId: order.tenantId, ...data } });
      await tx.tenant.update({ where: { id: order.tenantId }, data: { subscriptionPlanId: order.targetPlan.id, subscriptionStartedAt: now } });
      await this.recordEntitlement(tx, order, subscription, 'PLAN_UPGRADE', `升级至${order.targetPlan.name}`);
    } else if (order.orderType === 'RENEW') {
      if (!subscription) throw new BadRequestException('订阅不存在，不能续费');
      const plan = order.targetPlan ?? await tx.subscriptionPlan.findUnique({ where: { id: subscription.planId } });
      if (!plan) throw new BadRequestException('续费套餐不存在');
      const base = subscription.expiresAt > now ? subscription.expiresAt : now;
      const expiresAt = this.addMonths(base, order.durationMonths);
      subscription = await tx.tenantSubscription.update({ where: { id: subscription.id }, data: { status: 'ACTIVE', billingCycle: order.billingCycle, expiresAt, graceEndsAt: this.addDays(expiresAt, plan.gracePeriodDays) } });
      await this.recordEntitlement(tx, order, subscription, 'RENEWAL', `续费至${expiresAt.toLocaleDateString('zh-CN')}`);
    } else {
      if (!subscription || !order.addon) throw new BadRequestException('订阅或扩容包不存在');
      if (order.historyAddonMonths > 0) {
        if (subscription.expiresAt <= now) throw new BadRequestException('订阅已到期，不能开通历史扩展包，请联系财务处理');
        const activeMonths = historyRetention(await tx.subscriptionPlan.findUnique({ where: { id: subscription.planId } }), subscription, now).addonMonths;
        if (activeMonths + order.historyAddonMonths > 1200) throw new BadRequestException('历史扩展累计不能超过1200个月');
        subscription = await tx.tenantSubscription.update({ where: { id: subscription.id }, data: { addonHistoryMonths: activeMonths + order.historyAddonMonths, historyAddonExpiresAt: subscription.expiresAt } });
        await this.recordEntitlement(tx, order, subscription, 'EXPANSION', `历史保存延长${order.historyAddonMonths}个月，随当前订阅到期`, 'HISTORY', order.historyAddonMonths);
        return;
      }
      const delta = order.addon.quantity * order.addonQuantity;
      const fieldMap: Record<string, 'addonDeviceLimit' | 'addonPointLimit' | 'addonScadaProjectLimit' | 'addonUserLimit'> = { DEVICE: 'addonDeviceLimit', POINT: 'addonPointLimit', SCADA: 'addonScadaProjectLimit', USER: 'addonUserLimit' };
      const field = fieldMap[String(order.addon.resourceType)];
      if (!field) throw new BadRequestException('扩容包资源类型无效');
      subscription = await tx.tenantSubscription.update({ where: { id: subscription.id }, data: { [field]: { increment: delta } } });
      await this.recordEntitlement(tx, order, subscription, 'EXPANSION', `${order.addon.name} × ${order.addonQuantity}`, order.addon.resourceType, delta);
    }
  }

  private checkout(order: any, origin?: string) {
    const content = order.targetPlan?.name ?? order.addon?.name ?? '订阅服务';
    return this.alipay.checkout(order, `微控物联云平台-${this.orderTypeName(order.orderType)}-${content}`, this.alipayReturnUrl(order.id, origin));
  }

  private alipayReturnUrl(orderId: string, origin?: string) {
    const configured = process.env.ALIPAY_RETURN_URL?.trim();
    let target: URL | undefined;
    try {
      if (configured) target = new URL(configured);
      else if (origin && /^https?:\/\//i.test(origin)) target = new URL('/subscription', origin);
    } catch { target = undefined; }
    if (!target) return undefined;
    target.searchParams.set('alipayOrder', orderId);
    return target.toString();
  }

  private async applyAlipayResult(orderNo: string, result: AlipayTradeResult) {
    if (result.code && result.code !== '10000') {
      if (result.sub_code === 'ACQ.TRADE_NOT_EXIST') {
        const waiting = await this.prisma.subscriptionOrder.findUnique({ where: { orderNo }, include: this.orderInclude });
        if (!waiting) throw new NotFoundException('支付订单不存在');
        return waiting;
      }
      throw new BadRequestException(result.sub_msg || result.msg || '支付宝交易查询失败');
    }
    const paid = ['TRADE_SUCCESS', 'TRADE_FINISHED'].includes(result.trade_status ?? '');
    const closed = result.trade_status === 'TRADE_CLOSED';
    return this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT "id" FROM "SubscriptionOrder" WHERE "orderNo" = ${orderNo} FOR UPDATE`;
      const current = await tx.subscriptionOrder.findUnique({ where: { orderNo }, include: { targetPlan: true, addon: true, subscription: true, tenant: true } });
      if (!current || current.paymentMethod !== 'ALIPAY') throw new NotFoundException('支付宝订单不存在');
      if (current.status === 'APPROVED' && current.paymentStatus === 'TRADE_SUCCESS') return tx.subscriptionOrder.findUniqueOrThrow({ where: { id: current.id }, include: this.orderInclude });
      if (result.total_amount) {
        const paidCents = Math.round(Number(result.total_amount) * 100);
        if (!Number.isFinite(paidCents) || paidCents !== current.paymentAmountCents) throw new BadRequestException('支付宝实付金额与订单金额不一致');
      }
      if (paid) {
        const now = new Date();
        await this.activateOrder(tx, current, now);
        await tx.subscriptionNotification.upsert({
          where: { dedupeKey: `ORDER:${current.id}:ALIPAY_PAID` },
          create: { tenantId: current.tenantId, dedupeKey: `ORDER:${current.id}:ALIPAY_PAID`, type: 'ORDER_APPROVED', title: '支付宝支付成功，权益已生效', message: `订单 ${current.orderNo} 已支付并完成自动开通。` },
          update: {},
        });
        return tx.subscriptionOrder.update({ where: { id: current.id }, data: { status: 'APPROVED', paymentStatus: 'TRADE_SUCCESS', providerTradeNo: result.trade_no || current.providerTradeNo, paidAt: now, activatedAt: now, paymentFailureReason: null }, include: this.orderInclude });
      }
      if (closed) return tx.subscriptionOrder.update({ where: { id: current.id }, data: { status: 'PAYMENT_CLOSED', paymentStatus: 'TRADE_CLOSED' }, include: this.orderInclude });
      if (result.trade_status) return tx.subscriptionOrder.update({ where: { id: current.id }, data: { paymentStatus: result.trade_status }, include: this.orderInclude });
      return tx.subscriptionOrder.findUniqueOrThrow({ where: { id: current.id }, include: this.orderInclude });
    });
  }

  private async recordEntitlement(tx: Prisma.TransactionClient, order: any, subscription: any, changeType: string, description: string, resourceType?: string, delta = 0) {
    const effective = (base: number, addon: number) => base === 0 ? 0 : base + addon;
    await tx.entitlementChange.create({ data: {
      tenantId: order.tenantId, subscriptionId: subscription.id, orderId: order.id, changeType,
      deviceDelta: resourceType === 'DEVICE' ? delta : 0, pointDelta: resourceType === 'POINT' ? delta : 0,
      scadaProjectDelta: resourceType === 'SCADA' ? delta : 0, userDelta: resourceType === 'USER' ? delta : 0,
      historyMonthsDelta: resourceType === 'HISTORY' ? delta : 0,
      effectiveDeviceLimit: effective(subscription.baseDeviceLimit, subscription.addonDeviceLimit),
      effectivePointLimit: effective(subscription.basePointLimit, subscription.addonPointLimit),
      effectiveScadaLimit: effective(subscription.baseScadaProjectLimit, subscription.addonScadaProjectLimit),
      effectiveUserLimit: effective(subscription.baseUserLimit, subscription.addonUserLimit), description,
    } });
  }

  private findPlan(id: string) { return this.prisma.subscriptionPlan.findUnique({ where: { id } }).then(plan => { if (!plan) throw new NotFoundException('套餐不存在'); return plan; }); }
  private cleanPlan<T extends CreateSubscriptionPlanDto | UpdateSubscriptionPlanDto>(dto: T) { return { ...dto, ...(dto.code !== undefined ? { code: dto.code.trim().toUpperCase() } : {}), ...(dto.name !== undefined ? { name: dto.name.trim() } : {}), ...(dto.description !== undefined ? { description: dto.description.trim() || null } : {}), ...(dto.billingCycles !== undefined ? { billingCycles: [...new Set(dto.billingCycles)] } : {}) }; }
  private cleanAddon(dto: SaveQuotaAddonDto) {
    if (dto.resourceType === 'HISTORY' && dto.quantity !== 3) throw new BadRequestException('历史扩展包每份固定3个月');
    if (dto.resourceType === 'HISTORY' && !dto.pricePending && !dto.quoteOnly && dto.priceCents <= 0) throw new BadRequestException('请填写历史扩展包价格，或选择价格待定');
    return { ...dto, code: dto.code.trim().toUpperCase(), name: dto.name.trim(), description: dto.description?.trim() || null };
  }
  private validateHistoryPlan(days: number, months: number) {
    if (!((days > 0 && months === 0) || (days === 0 && months > 0))) throw new BadRequestException('历史保存期限请选择天或自然月，不能同时设置，也不能无限保存');
  }
  private tenantId(actor: AuthUser) { if (!actor.tenantId) throw new BadRequestException('当前账号未关联租户'); return actor.tenantId; }
  private voucherDirectory() { return resolve(process.env.PAYMENT_VOUCHER_DIR || join(process.cwd(), 'storage', 'payment-vouchers')); }
  private validateVoucher(file: VoucherFile) { const allowed = new Set(['image/jpeg', 'image/png', 'image/webp', 'application/pdf']); if (!allowed.has(file.mimetype)) throw new BadRequestException('付款凭证仅支持 JPG、PNG、WebP 或 PDF'); if (file.size > 8 * 1024 * 1024) throw new BadRequestException('付款凭证不能超过 8 MB'); }
  private safeExtension(mimeType: string, originalName: string) { const byMime: Record<string, string> = { 'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp', 'application/pdf': '.pdf' }; return byMime[mimeType] ?? extname(originalName).toLowerCase().slice(0, 8); }
  private orderNo() { const now = new Date(); const stamp = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}${String(now.getSeconds()).padStart(2, '0')}`; return `SO${stamp}${randomBytes(3).toString('hex').toUpperCase()}`; }
  private addMonths(date: Date, months: number) { return shiftHistoryMonths(date, months); }
  private addDays(date: Date, days: number) { return new Date(date.getTime() + days * 86_400_000); }
  private orderTypeName(type: string) { return ({ UPGRADE: '升级', RENEW: '续费', EXPANSION: '扩容' } as Record<string, string>)[type] ?? type; }
  private readonly orderInclude = {
    tenant: { select: { id: true, name: true } }, targetPlan: true, addon: true, paymentProofs: { orderBy: { createdAt: 'desc' as const } }, financeReviews: { orderBy: { createdAt: 'desc' as const } },
  };
}

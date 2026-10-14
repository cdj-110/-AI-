import { BadRequestException, ForbiddenException, Injectable, Logger, NotFoundException, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { DeviceLinkageRule, Prisma } from '@prisma/client';
import { randomUUID } from 'crypto';
import { Subscription } from 'rxjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';
import { DeviceCommandsService } from '../device-commands/device-commands.service';
import { DevicesService } from '../devices/devices.service';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { PrismaService } from '../prisma/prisma.service';
import { EnableDeviceLinkageDto, SaveDeviceLinkageDto } from './device-linkage.dto';
import { freshLinkageSample, hasLinkageCycle, linkageMatches } from './device-linkage.conditions';

type Sample = { deviceId: string; time: string; metrics: Record<string, unknown> };
@Injectable()
export class DeviceLinkageService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(DeviceLinkageService.name);
  private subscription?: Subscription;
  private queue: Promise<void> = Promise.resolve();
  private queued = 0;
  private stopped = false;
  constructor(private readonly prisma: PrismaService, private readonly devices: DevicesService,
    private readonly commands: DeviceCommandsService, private readonly logs: OperationLogsService) {}

  onModuleInit() {
    this.subscription = this.devices.liveTelemetryUpdates().subscribe(sample => {
      if (this.stopped) return;
      if (this.queued >= 200) { this.logger.warn('联动处理队列已满，丢弃当前通知；不会补发历史数据'); return; }
      this.queued++;
      this.queue = this.queue.then(() => this.stopped ? undefined : this.processSample(sample))
        .catch(error => this.logger.error(error instanceof Error ? error.message : '联动处理失败'))
        .finally(() => { this.queued--; });
    });
  }
  onModuleDestroy() { this.stopped = true; this.subscription?.unsubscribe(); }
  private tenant(actor: AuthUser) {
    if (actor.role !== 'TENANT_ADMIN' || !actor.tenantId) throw new ForbiddenException('仅租户管理员可管理设备联动');
    return actor.tenantId;
  }
  private async rule(actor: AuthUser, id: string) {
    const rule = await this.prisma.deviceLinkageRule.findFirst({ where: { id, tenantId: this.tenant(actor), deletedAt: null } });
    if (!rule) throw new NotFoundException('联动规则不存在');
    return rule;
  }
  private audit(actor: AuthUser, id: string, name: string, action: string) {
    return this.logs.record(actor, { module: '设备联动', action, targetType: 'DeviceLinkageRule', targetId: id, targetName: name });
  }
  private async source(actor: AuthUser, deviceId: string, metric: string, online = false) {
    const device = await this.prisma.device.findFirst({ where: { id: deviceId, tenantId: this.tenant(actor) }, include: { tenant: { select: cloudPolicyTenantSelect }, gateway: true, metrics: true } });
    if (!device || tenantBlocksCloud(device) || !device.cloudAccessEnabled || (device.gateway && (!device.gateway.cloudAccessEnabled || device.gateway.tenantId !== device.tenantId))) throw new BadRequestException('来源设备不存在或云连接不可用');
    if (!device.metrics.some(m => m.identifier === metric && m.enabled && !m.ignored && m.dataType === 'NUMBER')) throw new BadRequestException('来源需选择已启用的数值点位');
    if (online && (device.status !== 'ONLINE' || (device.gateway && device.gateway.status !== 'ONLINE') || (device.deviceType === 'GATEWAY_CHILD' && !device.gateway))) throw new BadRequestException('来源设备或网关离线');
    return device;
  }
  private async validate(actor: AuthUser, dto: SaveDeviceLinkageDto, online = false) {
    if (!dto.name.trim()) throw new BadRequestException('请填写规则名称');
    if (dto.sourceDeviceId === dto.targetDeviceId && dto.sourceMetric === dto.targetMetric) throw new BadRequestException('来源和目标不能是同一点位');
    const source = await this.source(actor, dto.sourceDeviceId, dto.sourceMetric, online);
    const caps = await this.commands.capabilities(actor, dto.targetDeviceId);
    const point = caps.points.find(p => p.identifier === dto.targetMetric);
    if (!point || (online && !caps.available)) throw new BadRequestException(caps.reason || '目标点位不可写');
    const value = dto.actionValue;
    if (point.dataType === 'BOOLEAN' ? typeof value !== 'boolean' : point.dataType === 'NUMBER' ? typeof value !== 'number' || !Number.isFinite(value) : point.dataType === 'STRING' ? typeof value !== 'string' || value.length > 2048 : true) throw new BadRequestException('指令值与目标点位类型不匹配');
    const target = await this.prisma.device.findFirst({ where: { id: dto.targetDeviceId, tenantId: this.tenant(actor) } });
    if (!target) throw new BadRequestException('目标设备不属于当前租户');
    return { source, target };
  }
  async list(actor: AuthUser, page = 1, keyword = '') {
    page = Math.max(1, Math.min(100000, Math.floor(Number.isFinite(page) ? page : 1)));
    const where = { tenantId: this.tenant(actor), deletedAt: null, ...(keyword ? { name: { contains: keyword.slice(0, 80), mode: 'insensitive' as const } } : {}) };
    const [items, total] = await Promise.all([this.prisma.deviceLinkageRule.findMany({ where, orderBy: { createdAt: 'desc' }, skip: (page - 1) * 20, take: 20 }), this.prisma.deviceLinkageRule.count({ where })]);
    return { items, total, page };
  }
  async save(actor: AuthUser, dto: SaveDeviceLinkageDto, id?: string) {
    const old = id ? await this.rule(actor, id) : null;
    if (old?.enabled) throw new BadRequestException('请先停用规则再编辑');
    const { source, target } = await this.validate(actor, dto);
    const ruleId = id || randomUUID();
    await this.audit(actor, ruleId, dto.name.trim(), id ? '修改规则' : '新增规则');
    const data = { ...dto, name: dto.name.trim(), actionValue: dto.actionValue as Prisma.InputJsonValue, sourceName: source.name, targetName: target.name, authorizedBy: actor.sub, enabled: false, enabledAt: null, met: false, lastSampleAt: null, lastError: null };
    return this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage:${ruleId}`}))::text`;
      if (old) {
        const current = await tx.deviceLinkageRule.findFirst({ where: { id: ruleId, tenantId: this.tenant(actor), deletedAt: null } });
        if (!current || current.enabled) throw new BadRequestException('规则状态已改变，请刷新');
        return tx.deviceLinkageRule.update({ where: { id: ruleId }, data: { ...data, revision: { increment: 1 } } });
      }
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage-limit:${actor.tenantId}`}))::text`;
      if (await tx.deviceLinkageRule.count({ where: { tenantId: this.tenant(actor), deletedAt: null } }) >= 100) throw new BadRequestException('第一版每租户最多100条联动规则');
      return tx.deviceLinkageRule.create({ data: { id: ruleId, tenantId: this.tenant(actor), ...data } });
    });
  }
  private async unresolved(targetDeviceId: string, targetMetric: string, db: Prisma.TransactionClient = this.prisma) {
    const reserving = await db.deviceLinkageRun.findMany({ where: { targetDeviceId, targetMetric, status: 'DISPATCHING' }, take: 100 });
    for (const run of reserving) {
      const command = await db.deviceCommand.findUnique({ where: { requestId: run.requestId } });
      if (!command) return true; // A crash between reservation and dispatch is never retried automatically.
      await db.deviceLinkageRun.update({ where: { id: run.id }, data: { status: command.status, message: command.message } });
    }
    return !!await db.deviceCommand.findFirst({ where: { deviceId: targetDeviceId, metric: targetMetric, status: { in: ['SENT', 'TIMEOUT'] } } });
  }
  async enable(actor: AuthUser, id: string, dto: EnableDeviceLinkageDto) {
    const rule = await this.rule(actor, id);
    if (dto.enabled) {
      if (!dto.confirmAutomaticControl) throw new BadRequestException('启用前必须确认自动下发授权');
      await this.validate(actor, rule, true);
      if (await this.unresolved(rule.targetDeviceId, rule.targetMetric)) throw new BadRequestException('目标点位存在待确认指令，请核实回执，不能重复下发');
    }
    await this.audit(actor, id, rule.name, dto.enabled ? '授权并启用自动联动' : '停用联动');
    return this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage-enable:${actor.tenantId}`}))::text`;
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage:${id}`}))::text`;
      const current = await tx.deviceLinkageRule.findFirst({ where: { id, tenantId: this.tenant(actor), deletedAt: null } });
      if (!current || current.revision !== rule.revision) throw new BadRequestException('规则已变更，请重新确认');
      if (current.enabled === dto.enabled) return current;
      if (dto.enabled && hasLinkageCycle([...await tx.deviceLinkageRule.findMany({ where: { tenantId: this.tenant(actor), enabled: true, deletedAt: null, id: { not: id } } }), current])) throw new BadRequestException('启用后形成闭环联动，第一版不允许循环控制');
      return tx.deviceLinkageRule.update({ where: { id }, data: { enabled: dto.enabled, enabledAt: dto.enabled ? new Date() : null, authorizedBy: actor.sub, revision: { increment: 1 }, met: false, lastSampleAt: null, lastError: null } });
    });
  }
  async remove(actor: AuthUser, id: string) {
    const rule = await this.rule(actor, id);
    if (rule.enabled) throw new BadRequestException('请先停用规则再删除');
    await this.audit(actor, id, rule.name, '删除联动规则');
    const result = await this.prisma.deviceLinkageRule.updateMany({ where: { id, tenantId: this.tenant(actor), deletedAt: null, enabled: false }, data: { deletedAt: new Date(), revision: { increment: 1 } } });
    if (!result.count) throw new BadRequestException('规则状态已改变，请刷新');
    return { removed: true };
  }
  async runs(actor: AuthUser, id: string, page = 1) {
    await this.rule(actor, id);
    page = Math.max(1, Math.min(100000, Math.floor(Number.isFinite(page) ? page : 1)));
    const [items, total] = await Promise.all([this.prisma.deviceLinkageRun.findMany({ where: { ruleId: id }, orderBy: { createdAt: 'desc' }, skip: (page - 1) * 20, take: 20 }), this.prisma.deviceLinkageRun.count({ where: { ruleId: id } })]);
    const commands = await this.prisma.deviceCommand.findMany({ where: { requestId: { in: items.map(r => r.requestId) } } });
    return { items: items.map(run => { const command = commands.find(c => c.requestId === run.requestId); return { ...run, status: command?.status || run.status, message: command?.message || run.message, commandId: command?.id }; }), total, page };
  }
  private async executionActor(rule: DeviceLinkageRule): Promise<AuthUser> {
    const user = await this.prisma.user.findUnique({ where: { id: rule.authorizedBy }, include: { tenant: true } });
    if (!user || user.role !== 'TENANT_ADMIN' || user.status !== 'ACTIVE' || user.tenantId !== rule.tenantId || user.tenant?.status !== 'ACTIVE' || user.tenant.deviceCloudAccessDisabled) throw new ForbiddenException('授权管理员或租户不可用');
    return { sub: user.id, sid: 'cloud-linkage', uv: user.updatedAt.getTime(), username: user.username, role: user.role, tenantId: user.tenantId };
  }
  async processSample(sample: Sample) {
    if (!sample || typeof sample.deviceId !== 'string' || !sample.metrics || typeof sample.metrics !== 'object') return;
    const rules = await this.prisma.deviceLinkageRule.findMany({ where: { sourceDeviceId: sample.deviceId, enabled: true, deletedAt: null } });
    for (const candidate of rules) {
      try { await this.evaluate(candidate, sample); }
      catch (error) { this.logger.error(`规则 ${candidate.id}: ${error instanceof Error ? error.message : '执行异常'}`); }
    }
  }
  private async evaluate(candidate: DeviceLinkageRule, sample: Sample) {
    if (!freshLinkageSample(sample.time, candidate.enabledAt, candidate.lastSampleAt)) return;
    const value = sample.metrics[candidate.sourceMetric];
    if (linkageMatches(value, candidate.operator, candidate.threshold) === undefined) return;
    let actor: AuthUser;
    try { actor = await this.executionActor(candidate); await this.source(actor, candidate.sourceDeviceId, candidate.sourceMetric, true); }
    catch (error) {
      await this.prisma.deviceLinkageRule.updateMany({ where: { id: candidate.id, revision: candidate.revision }, data: { enabled: false, lastError: error instanceof Error ? error.message : '来源不可用' } }); return;
    }
    const lastRun = await this.prisma.deviceLinkageRun.findFirst({ where: { ruleId: candidate.id, sampleAt: { gt: candidate.enabledAt! }, status: { in: ['DISPATCHING', 'SENT', 'TIMEOUT', 'FAILED', 'SUCCEEDED'] } }, orderBy: { createdAt: 'desc' } });
    const lastCommand = lastRun ? await this.prisma.deviceCommand.findUnique({ where: { requestId: lastRun.requestId } }) : null;
    if (lastCommand && ['FAILED', 'TIMEOUT'].includes(lastCommand.status)) {
      await this.prisma.deviceLinkageRule.updateMany({ where: { id: candidate.id, revision: candidate.revision }, data: { enabled: false, lastError: lastCommand.message || '上次指令失败或结果未知，已停用规则，请核实后重新授权' } }); return;
    }
    const run = await this.prisma.$transaction(async tx => {
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage:${candidate.id}`}))::text`;
      const rule = await tx.deviceLinkageRule.findUnique({ where: { id: candidate.id } });
      if (!rule || !rule.enabled || rule.deletedAt || rule.revision !== candidate.revision || !freshLinkageSample(sample.time, rule.enabledAt, rule.lastSampleAt)) return null;
      const met = linkageMatches(value, rule.operator, rule.threshold)!;
      await tx.deviceLinkageRule.update({ where: { id: rule.id }, data: { met, lastSampleAt: new Date(sample.time) } });
      if (!met || rule.met) return null;
      await tx.$queryRaw`SELECT pg_advisory_xact_lock(hashtext(${`linkage-target:${rule.targetDeviceId}:${rule.targetMetric}`}))::text`;
      const blocked = await this.unresolved(rule.targetDeviceId, rule.targetMetric, tx);
      const recent = await tx.deviceLinkageRun.findFirst({ where: { targetDeviceId: rule.targetDeviceId, targetMetric: rule.targetMetric, status: { in: ['DISPATCHING', 'SENT', 'SUCCEEDED', 'TIMEOUT'] }, createdAt: { gt: new Date(Date.now() - 30_000) } } });
      const result = await tx.deviceLinkageRun.create({ data: { ruleId: rule.id, requestId: randomUUID(), targetDeviceId: rule.targetDeviceId, targetMetric: rule.targetMetric, observedValue: value as number, actionValue: rule.actionValue as Prisma.InputJsonValue, sampleAt: new Date(sample.time), status: blocked ? 'SKIPPED' : recent ? 'THROTTLED' : 'DISPATCHING', message: blocked ? '目标点位存在待确认指令，规则已停用' : recent ? '同一目标点位30秒内不再次下发；本次触发不补发' : '已预留一次下发，未确认时不会自动重试' } });
      if (blocked) await tx.deviceLinkageRule.update({ where: { id: rule.id }, data: { enabled: false, lastError: result.message } });
      return result;
    });
    if (!run || run.status !== 'DISPATCHING') return;
    try {
      const current = await this.prisma.deviceLinkageRule.findUnique({ where: { id: candidate.id } });
      if (this.stopped || !current?.enabled || current.deletedAt || current.revision !== candidate.revision || !freshLinkageSample(sample.time, current.enabledAt, null)) {
        await this.prisma.deviceLinkageRun.update({ where: { id: run.id }, data: { status: 'CANCELLED', message: '规则已停用或变更，指令未发送' } }); return;
      }
      actor = await this.executionActor(current);
      await this.source(actor, current.sourceDeviceId, current.sourceMetric, true);
      const command = await this.commands.create(actor, run.targetDeviceId, { requestId: run.requestId, metric: run.targetMetric, value: run.actionValue });
      await this.prisma.deviceLinkageRun.update({ where: { id: run.id }, data: { status: command.status, message: command.message } });
      if (command.status === 'FAILED') await this.prisma.deviceLinkageRule.updateMany({ where: { id: current.id, revision: current.revision }, data: { enabled: false, lastError: command.message || '指令失败，请检查后重新启用' } });
    } catch (error) {
      // Persisted reservation stays locked if command outcome cannot be verified.
      const existing = await this.prisma.deviceCommand.findUnique({ where: { requestId: run.requestId } });
      const message = error instanceof Error ? error.message : '下发异常';
      await this.prisma.deviceLinkageRun.update({ where: { id: run.id }, data: { status: existing?.status || 'FAILED', message: existing?.message || message } });
      await this.prisma.deviceLinkageRule.updateMany({ where: { id: candidate.id, revision: candidate.revision }, data: { enabled: false, lastError: message } });
    }
  }
}

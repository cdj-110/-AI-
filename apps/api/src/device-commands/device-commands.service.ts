import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { DeviceCommand, Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { CreateDeviceCommandDto } from './create-device-command.dto';
import { DeviceCommandsMqttService } from './device-commands-mqtt.service';
import { gatewayPointAccess } from '../common/gateway-point-access';
import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';

@Injectable()
export class DeviceCommandsService {
  constructor(private readonly prisma: PrismaService, private readonly mqtt: DeviceCommandsMqttService, private readonly logs: OperationLogsService) {}

  private async device(actor: AuthUser, id: string) {
    if (!['SUPER_ADMIN', 'TENANT_ADMIN'].includes(actor.role) || (actor.role !== 'SUPER_ADMIN' && !actor.tenantId)) throw new ForbiddenException('没有点位下发权限');
    const device = await this.prisma.device.findFirst({ where: { id, ...(actor.role === 'SUPER_ADMIN' ? {} : { tenantId: actor.tenantId! }) }, include: { tenant: { select: cloudPolicyTenantSelect }, gateway: true, metrics: true } });
    if (!device) throw new NotFoundException('设备不存在');
    return device;
  }

  async capabilities(actor: AuthUser, id: string) {
    const device = await this.device(actor, id), gateway = device.gateway;
    if (tenantBlocksCloud(device)) return { points: [], available: false, reason: '所属租户已停用设备云连接' };
    if (!device.cloudAccessEnabled) return { points: [], available: false, reason: '该设备的云连接已被管理平台禁用' };
    if (gateway && !gateway.cloudAccessEnabled) return { points: [], available: false, reason: '所属网关的云连接已被管理平台禁用' };
    if (device.deviceType !== 'GATEWAY_CHILD' || !gateway || gateway.tenantId !== device.tenantId) return { points: [], available: false, reason: '当前仅支持已绑定网关的采集子设备' };
    const draft = await this.prisma.gatewayTopologyDraft.findUnique({ where: { gatewayDeviceId: gateway.id }, select: { items: true } });
    const supported = gatewayPointAccess(gateway.reportedConfig, draft?.items, device.deviceKey);
    if (supported === null) return { points: [], available: false, reason: '请更新 EG100 并重新读取网关配置，以启用云端下发' };
    const points = device.metrics.filter(metric => metric.enabled && !metric.ignored && supported.some(point => point.canWrite && point.metric === metric.identifier))
      .map(metric => ({ identifier: metric.identifier, name: metric.name, dataType: supported.find(point => point.metric === metric.identifier)?.dataType || metric.dataType, unit: metric.unit }))
      .filter(point => ['NUMBER', 'BOOLEAN', 'STRING'].includes(point.dataType));
    const online = gateway.cloudAccessEnabled && device.cloudAccessEnabled && gateway.status === 'ONLINE' && device.status === 'ONLINE';
    return { points, available: online && points.length > 0, reason: !online ? '设备或网关离线，暂时不能下发' : !points.length ? '网关当前没有上报可写点位' : '', gatewayId: gateway.id };
  }

  async list(actor: AuthUser, id: string) {
    await this.device(actor, id);
    await this.mqtt.expire();
    return this.prisma.deviceCommand.findMany({ where: { deviceId: id }, orderBy: { createdAt: 'desc' }, take: 30 });
  }

  async create(actor: AuthUser, id: string, dto: CreateDeviceCommandDto, request: { ip?: string; userAgent?: string } = {}): Promise<DeviceCommand> {
    const device = await this.device(actor, id);
    // A reused request ID returns its original result; it can never execute twice.
    const existing = await this.prisma.deviceCommand.findUnique({ where: { requestId: dto.requestId } });
    if (existing) {
      if (existing.deviceId !== id || existing.createdBy !== actor.sub || existing.metric !== dto.metric || JSON.stringify(existing.value) !== JSON.stringify(dto.value)) throw new BadRequestException('请求编号已被其他下发操作使用');
      await this.mqtt.expire();
      return this.prisma.deviceCommand.findUniqueOrThrow({ where: { id: existing.id } });
    }
    const caps = await this.capabilities(actor, id);
    if (!caps.available) throw new BadRequestException(caps.reason);
    if (caps.gatewayId !== device.gateway?.id) throw new BadRequestException('设备归属网关已改变，请刷新后重新确认');
    const point = caps.points.find(item => item.identifier === dto.metric);
    if (!point) throw new BadRequestException('该点位未启用写入或网关不支持写入');
    if (point.dataType === 'NUMBER' ? typeof dto.value !== 'number' || !Number.isFinite(dto.value) : point.dataType === 'BOOLEAN' ? typeof dto.value !== 'boolean' : point.dataType === 'STRING' ? typeof dto.value !== 'string' || dto.value.length > 2048 : true) throw new BadRequestException('下发值的类型或长度不正确');
    const createdAt = new Date(), expiresAt = new Date(createdAt.getTime() + 30_000);
    let task;
    try {
      task = await this.prisma.deviceCommand.create({ data: { requestId: dto.requestId, deviceId: id, gatewayId: device.gateway!.id, deviceKey: device.deviceKey, metric: dto.metric, value: dto.value as Prisma.InputJsonValue, createdBy: actor.sub, createdAt, expiresAt } });
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2002') return this.create(actor, id, dto, request);
      throw error;
    }
    try {
      // Audit before publishing. Failure to record means no device write occurs.
      await this.logs.record(actor, { module: '设备下发', action: '下发点位值', targetType: 'DeviceCommand', targetId: task.id, targetName: device.name, ...request, detail: { gatewayId: task.gatewayId, deviceId: id, deviceKey: device.deviceKey, metric: dto.metric, value: dto.value, requestId: dto.requestId } });
    } catch {
      return this.prisma.deviceCommand.update({ where: { id: task.id }, data: { status: 'FAILED', message: '操作审计记录失败，指令未发送', completedAt: new Date() } });
    }
    try { await this.mqtt.publish(device.gateway!.deviceKey, task); }
    catch (error) {
      // A publish timeout is ambiguous, so never claim failure or auto retry.
      const neverSent = !!error && typeof error === 'object' && 'neverSent' in error;
      await this.prisma.deviceCommand.updateMany({ where: { id: task.id, status: 'SENT' }, data: neverSent ? { status: 'FAILED', message: '下发通道未就绪，指令未发送', completedAt: new Date() } : { message: '发送状态未确认，正在等待网关回执；请勿重复下发' } });
    }
    return this.prisma.deviceCommand.findUniqueOrThrow({ where: { id: task.id } });
  }
}

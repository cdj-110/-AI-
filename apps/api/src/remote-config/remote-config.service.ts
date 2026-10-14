import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateRemoteConfigDto } from './dto/create-remote-config.dto';
import { RemoteConfigMqttService } from './remote-config-mqtt.service';
import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';
import { deviceAccessWhere } from '../common/user-permissions';

@Injectable()
export class RemoteConfigService {
  constructor(private readonly prisma: PrismaService, private readonly mqtt: RemoteConfigMqttService) {}

  async list(actor: AuthUser, deviceId: string) {
    const device = await this.gateway(actor, deviceId);
    await this.prisma.remoteConfigTask.updateMany({
      where: { deviceId: device.id, status: 'SENT', sentAt: { lt: new Date(Date.now() - 2 * 60 * 1000) } },
      data: { status: 'TIMEOUT', error: '网关在 2 分钟内未返回执行结果', completedAt: new Date() },
    });
    return this.prisma.remoteConfigTask.findMany({ where: { deviceId: device.id }, orderBy: { version: 'desc' }, take: 50 });
  }

  async create(actor: AuthUser, deviceId: string, dto: CreateRemoteConfigDto) {
    const device = await this.gateway(actor, deviceId);
    this.ensureCloudAccess(device);
    const latest = await this.prisma.remoteConfigTask.findFirst({ where: { deviceId }, orderBy: { version: 'desc' }, select: { version: true } });
    const task = await this.prisma.remoteConfigTask.create({
      data: {
        deviceId,
        version: (latest?.version ?? 0) + 1,
        config: dto.config as Prisma.InputJsonValue,
        createdBy: actor.username,
      },
    });
    return this.send(device.deviceKey, task.id);
  }

  async current(actor: AuthUser, deviceId: string) {
    const device = await this.gateway(actor, deviceId);
    this.ensureCloudAccess(device);
    try {
      return await this.mqtt.requestConfig(device.deviceKey);
    } catch (error) {
      if (device.reportedConfig) return { config: device.reportedConfig, reportedAt: device.reportedConfigAt, stale: true };
      throw new BadRequestException(error instanceof Error ? error.message : String(error));
    }
  }

  async retry(actor: AuthUser, deviceId: string, taskId: string) {
    const device = await this.gateway(actor, deviceId);
    this.ensureCloudAccess(device);
    const task = await this.prisma.remoteConfigTask.findFirst({ where: { id: taskId, deviceId } });
    if (!task) throw new NotFoundException('远程配置任务不存在');
    return this.send(device.deviceKey, task.id);
  }

  private async send(deviceKey: string, taskId: string) {
    const task = await this.prisma.remoteConfigTask.update({
      where: { id: taskId },
      data: { status: 'SENT', error: null, sentAt: new Date(), completedAt: null },
    });
    try {
      await this.mqtt.publish(deviceKey, task);
      return this.prisma.remoteConfigTask.findUniqueOrThrow({ where: { id: task.id } });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      await this.prisma.remoteConfigTask.update({ where: { id: task.id }, data: { status: 'FAILED', error: message, completedAt: new Date() } });
      throw new BadRequestException(message);
    }
  }

  private async gateway(actor: AuthUser, id: string) {
    const device = await this.prisma.device.findFirst({ where: { AND: [{ id }, deviceAccessWhere(actor)] }, include: { tenant: { select: cloudPolicyTenantSelect } } });
    if (!device) throw new NotFoundException('设备不存在');
    if (device.deviceType !== 'GATEWAY') throw new BadRequestException('只有网关设备支持远程配置');
    return device;
  }

  private ensureCloudAccess(device: { cloudAccessEnabled: boolean; tenant?: { deviceCloudAccessDisabled: boolean } }) {
    if (tenantBlocksCloud(device)) throw new BadRequestException('所属租户已停用设备云连接');
    if (!device.cloudAccessEnabled) throw new BadRequestException('该网关的云连接已被管理平台禁用');
  }
}

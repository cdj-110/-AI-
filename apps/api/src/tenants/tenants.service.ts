import { Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { CreateTenantDto } from './dto/create-tenant.dto';
import { UpdateTenantDto } from './dto/update-tenant.dto';
import { TenantAccountsQueryDto } from './dto/tenant-accounts-query.dto';
import { MqttConnectionAdminService } from '../mqtt/mqtt-connection-admin.service';
import { RemoteAccessGateway } from '../remote-access/remote-access.gateway';

@Injectable()
export class TenantsService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly mqtt: MqttConnectionAdminService,
    private readonly remoteAccess: RemoteAccessGateway,
  ) {}

  async findAll() {
    const tenants = await this.prisma.tenant.findMany({
      include: {
        subscriptionPlan: { select: { id: true, name: true, code: true } },
        subscription: { select: { status: true, startedAt: true, expiresAt: true, graceEndsAt: true } },
        _count: { select: { users: true, devices: true } },
        users: {
          where: { role: 'TENANT_ADMIN' },
          orderBy: [{ createdAt: 'asc' }, { id: 'asc' }],
          select: { id: true, username: true, nickname: true, email: true, phone: true, status: true },
        },
      },
      orderBy: { createdAt: 'desc' },
    });
    return tenants.map(({ users, ...tenant }) => ({ ...tenant, primaryAccounts: users }));
  }

  async create(dto: CreateTenantDto) {
    const defaultPlan = await this.prisma.subscriptionPlan.findFirst({
      where: { enabled: true },
      orderBy: [{ priceCents: 'asc' }, { sortOrder: 'asc' }, { createdAt: 'asc' }],
      select: { id: true, deviceLimit: true, pointLimit: true, scadaProjectLimit: true, userLimit: true, gracePeriodDays: true },
    });
    const now = new Date();
    const expiresAt = new Date(now); expiresAt.setUTCFullYear(expiresAt.getUTCFullYear() + 1);
    const graceEndsAt = new Date(expiresAt.getTime() + (defaultPlan?.gracePeriodDays ?? 7) * 86_400_000);
    return this.prisma.tenant.create({ data: {
      ...dto,
      deviceCloudAccessDisabled: dto.status === 'DISABLED' && dto.deviceCloudAccessDisabled === true,
      subscriptionPlanId: defaultPlan?.id,
      subscriptionStartedAt: defaultPlan ? new Date() : undefined,
      ...(defaultPlan ? { subscription: { create: {
        planId: defaultPlan.id, status: 'ACTIVE', billingCycle: 'YEARLY', startedAt: now, expiresAt, graceEndsAt,
        baseDeviceLimit: defaultPlan.deviceLimit, basePointLimit: defaultPlan.pointLimit,
        baseScadaProjectLimit: defaultPlan.scadaProjectLimit, baseUserLimit: defaultPlan.userLimit,
      } } } : {}),
    } });
  }

  async findOne(id: string) {
    const tenant = await this.prisma.tenant.findUnique({
      where: { id },
      include: { _count: { select: { users: true, devices: true } } },
    });
    if (!tenant) throw new NotFoundException('租户不存在');
    return tenant;
  }

  async update(id: string, dto: UpdateTenantDto) {
    // Serialize policy edits; never overwrite the per-device ban when blocking a tenant.
    const tenant = await this.prisma.$transaction(async (tx) => {
      await tx.$queryRaw`SELECT "id" FROM "Tenant" WHERE "id" = ${id} FOR UPDATE`;
      const current = await tx.tenant.findUnique({ where: { id } });
      if (!current) throw new NotFoundException('租户不存在');
      const disabled = (dto.status ?? current.status) === 'DISABLED'
        && (dto.deviceCloudAccessDisabled ?? current.deviceCloudAccessDisabled);
      const updated = await tx.tenant.update({ where: { id }, data: { ...dto, deviceCloudAccessDisabled: disabled } });
      if (disabled) {
        await tx.device.updateMany({ where: { tenantId: id, status: 'ONLINE' }, data: { status: 'OFFLINE' } });
      }
      return updated;
    });
    const disconnectResults: Array<{ clientId: string; result: string }> = [];
    if (tenant.deviceCloudAccessDisabled) {
      const devices = await this.prisma.device.findMany({
        where: { tenantId: id },
        select: { id: true, mqttClientId: true, factoryGateway: { select: { hardwareId: true } } },
      });
      const clientIds = new Set<string>();
      for (const device of devices) {
        this.remoteAccess.disconnectDevice(device.id);
        clientIds.add(device.mqttClientId);
        if (device.factoryGateway) clientIds.add(`factory_${device.factoryGateway.hardwareId}`);
      }
      // Bounded concurrency avoids flooding the broker for large tenants.
      const pending = [...clientIds];
      await Promise.all(Array.from({ length: Math.min(5, pending.length) }, async () => {
        let clientId: string | undefined;
        while ((clientId = pending.pop()) !== undefined) {
          disconnectResults.push({ clientId, result: await this.mqtt.disconnect(clientId) });
        }
      }));
    }
    return { ...tenant, disconnectResults };
  }

  async accounts(tenantId: string, query: TenantAccountsQueryDto) {
    const tenant = await this.prisma.tenant.findUnique({ where: { id: tenantId }, select: { id: true } });
    if (!tenant) throw new NotFoundException('租户不存在');
    const keyword = query.keyword?.trim();
    const where: Prisma.UserWhereInput = {
      tenantId,
      role: query.role || { in: ['TENANT_ADMIN', 'TENANT_USER'] },
      ...(keyword ? { OR: [
        { username: { contains: keyword, mode: 'insensitive' as const } },
        { nickname: { contains: keyword, mode: 'insensitive' as const } },
        { email: { contains: keyword, mode: 'insensitive' as const } },
        { phone: { contains: keyword } },
      ] } : {}),
    };
    const [users, total] = await this.prisma.$transaction([
      this.prisma.user.findMany({
        where,
        skip: (query.page - 1) * query.pageSize,
        take: query.pageSize,
        orderBy: [{ role: 'asc' }, { createdAt: 'asc' }, { id: 'asc' }],
        select: {
          id: true, username: true, nickname: true, email: true, phone: true,
          role: true, status: true, createdAt: true,
          loginLogs: { where: { success: true }, orderBy: { createdAt: 'desc' }, take: 1, select: { createdAt: true } },
        },
      }),
      this.prisma.user.count({ where }),
    ]);
    return {
      items: users.map(({ loginLogs, ...user }) => ({ ...user, lastLoginAt: loginLogs[0]?.createdAt ?? null })),
      total, page: query.page, pageSize: query.pageSize,
    };
  }

  async remove(id: string) {
    await this.findOne(id);
    await this.prisma.tenant.delete({ where: { id } });
    return { message: '删除成功' };
  }
}

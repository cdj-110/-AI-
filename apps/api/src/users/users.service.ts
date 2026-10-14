import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma, User } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { DEFAULT_TENANT_USER_PERMISSIONS, normalizeTenantUserPermissions } from '../common/user-permissions';
import { PrismaService } from '../prisma/prisma.service';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { SubscriptionLimitsService } from '../subscriptions/subscription-limits.service';

type UserWithScope = User & {
  deviceAccesses?: Array<{ deviceId: string }>;
  deviceGroupAccesses?: Array<{ groupId: string }>;
};

@Injectable()
export class UsersService {
  constructor(private readonly prisma: PrismaService, private readonly subscriptionLimits: SubscriptionLimitsService) {}

  async findAll(actor: AuthUser, page: number, pageSize: number, keyword: string) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.UserWhereInput = {
      ...(actor.role === 'TENANT_ADMIN'
        ? { tenantId: actor.tenantId }
        : { tenantId: null, role: 'SUPER_ADMIN' }),
      ...(keyword
        ? { OR: [
          { username: { contains: keyword, mode: 'insensitive' } },
          { nickname: { contains: keyword, mode: 'insensitive' } },
          { email: { contains: keyword, mode: 'insensitive' } },
          { phone: { contains: keyword } },
        ] }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.user.findMany({ where, skip: (safePage - 1) * safePageSize, take: safePageSize, orderBy: { createdAt: 'desc' }, include: this.scopeInclude }),
      this.prisma.user.count({ where }),
    ]);
    return { items: items.map(this.toPublicUser), total, page: safePage, pageSize: safePageSize };
  }

  async create(actor: AuthUser, dto: CreateUserDto) {
    this.assertWritableRole(actor, dto.role);
    const tenantId = actor.role === 'TENANT_ADMIN' ? actor.tenantId : null;
    if (dto.role !== 'SUPER_ADMIN' && !tenantId) throw new BadRequestException('租户用户必须指定 tenantId');
    if (tenantId) await this.subscriptionLimits.assertCanAddUsers(tenantId);
    const username = dto.username.trim();
    if (!username) throw new BadRequestException('请输入用户名称');
    const email = dto.email?.trim().toLowerCase() || null;
    const phone = dto.phone?.trim() || null;
    this.assertLoginCredential(email, phone);
    await this.assertCredentialAvailable(email, phone);
    const scopeMode = dto.role === 'TENANT_USER' ? dto.deviceScopeMode ?? 'ALL' : 'ALL';
    const deviceIds = scopeMode === 'CUSTOM' ? dto.deviceIds ?? [] : [];
    const groupIds = scopeMode === 'CUSTOM' ? dto.groupIds ?? [] : [];
    if (tenantId && dto.role === 'TENANT_USER') await this.validateScope(tenantId, scopeMode, deviceIds, groupIds);
    const user = await this.prisma.user.create({
      data: {
        username,
        nickname: username,
        email,
        phone,
        tenantId,
        password: await bcrypt.hash(dto.password, 10),
        role: dto.role,
        status: dto.status ?? 'ACTIVE',
        permissions: dto.role === 'TENANT_USER'
          ? normalizeTenantUserPermissions(dto.permissions ?? DEFAULT_TENANT_USER_PERMISSIONS)
          : [],
        deviceScopeMode: scopeMode,
        ...(deviceIds.length ? { deviceAccesses: { create: deviceIds.map(deviceId => ({ deviceId })) } } : {}),
        ...(groupIds.length ? { deviceGroupAccesses: { create: groupIds.map(groupId => ({ groupId })) } } : {}),
      },
      include: this.scopeInclude,
    });
    return this.toPublicUser(user);
  }

  async deviceScopeOptions(actor: AuthUser) {
    if (actor.role !== 'TENANT_ADMIN' || !actor.tenantId) throw new ForbiddenException('只有租户管理员可以分配设备范围');
    const [groups, devices] = await this.prisma.$transaction([
      this.prisma.deviceGroup.findMany({
        where: { tenantId: actor.tenantId },
        select: { id: true, name: true, _count: { select: { deviceMemberships: true } } },
        orderBy: [{ name: 'asc' }, { id: 'asc' }],
      }),
      this.prisma.device.findMany({
        where: { tenantId: actor.tenantId },
        select: { id: true, name: true, deviceKey: true, deviceType: true, groupMemberships: { select: { groupId: true } } },
        orderBy: [{ name: 'asc' }, { id: 'asc' }],
        take: 2000,
      }),
    ]);
    return {
      groups: groups.map(({ _count, ...group }) => ({ ...group, _count: { devices: _count.deviceMemberships } })),
      devices: devices.map(({ groupMemberships, ...device }) => ({ ...device, groupIds: groupMemberships.map(item => item.groupId) })),
    };
  }

  async findOne(actor: AuthUser, id: string) {
    const user = await this.getAccessibleUser(actor, id);
    return this.toPublicUser(user);
  }

  async update(actor: AuthUser, id: string, dto: UpdateUserDto) {
    const target = await this.getAccessibleUser(actor, id);
    if (dto.role) this.assertWritableRole(actor, dto.role);
    if (actor.role === 'TENANT_ADMIN' && dto.tenantId && dto.tenantId !== actor.tenantId) {
      throw new ForbiddenException('不能移动其他租户的用户');
    }
    const { password, permissions, deviceScopeMode, deviceIds, groupIds, ...rest } = dto;
    if (dto.username !== undefined && !dto.username.trim()) throw new BadRequestException('请输入用户名称');
    const email = dto.email === undefined ? target.email : dto.email.trim().toLowerCase() || null;
    const phone = dto.phone === undefined ? target.phone : dto.phone.trim() || null;
    this.assertLoginCredential(email, phone);
    await this.assertCredentialAvailable(email, phone, target.id);
    const nextScopeMode = target.role === 'TENANT_USER' ? deviceScopeMode ?? target.deviceScopeMode as 'ALL' | 'CUSTOM' : 'ALL';
    const nextDeviceIds = nextScopeMode === 'CUSTOM' ? deviceIds ?? target.deviceAccesses?.map(item => item.deviceId) ?? [] : [];
    const nextGroupIds = nextScopeMode === 'CUSTOM' ? groupIds ?? target.deviceGroupAccesses?.map(item => item.groupId) ?? [] : [];
    if (target.tenantId && target.role === 'TENANT_USER') await this.validateScope(target.tenantId, nextScopeMode, nextDeviceIds, nextGroupIds);
    const user = await this.prisma.$transaction(async tx => {
      if (target.role === 'TENANT_USER' && (deviceScopeMode !== undefined || deviceIds !== undefined || groupIds !== undefined)) {
        await tx.userDeviceAccess.deleteMany({ where: { userId: target.id } });
        await tx.userDeviceGroupAccess.deleteMany({ where: { userId: target.id } });
        if (nextDeviceIds.length) await tx.userDeviceAccess.createMany({ data: nextDeviceIds.map(deviceId => ({ userId: target.id, deviceId })) });
        if (nextGroupIds.length) await tx.userDeviceGroupAccess.createMany({ data: nextGroupIds.map(groupId => ({ userId: target.id, groupId })) });
      }
      return tx.user.update({
        where: { id: target.id },
        data: {
          ...rest,
          ...(dto.username !== undefined ? { username: dto.username.trim(), nickname: dto.username.trim() } : {}),
          email,
          phone,
          ...(password ? { password: await bcrypt.hash(password, 10) } : {}),
          ...(target.role === 'TENANT_USER'
            ? { permissions: normalizeTenantUserPermissions(permissions ?? target.permissions), deviceScopeMode: nextScopeMode }
            : {}),
        },
        include: this.scopeInclude,
      });
    });
    return this.toPublicUser(user);
  }

  async remove(actor: AuthUser, id: string) {
    const target = await this.getAccessibleUser(actor, id);
    if (actor.sub === id) throw new BadRequestException('不能删除当前登录用户');
    await this.prisma.user.delete({ where: { id: target.id } });
    return { message: '删除成功' };
  }

  private async getAccessibleUser(actor: AuthUser, id: string) {
    const user = await this.prisma.user.findUnique({ where: { id }, include: this.scopeInclude });
    if (!user) throw new NotFoundException('用户不存在');
    if (actor.role === 'TENANT_ADMIN' && user.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该用户');
    if (actor.role === 'TENANT_ADMIN' && user.role !== 'TENANT_USER') {
      throw new ForbiddenException('租户管理员只能维护普通成员账号');
    }
    if (actor.role === 'SUPER_ADMIN' && (user.tenantId || user.role !== 'SUPER_ADMIN')) {
      throw new ForbiddenException('平台账号管理不能操作租户账号');
    }
    return user;
  }

  private assertWritableRole(actor: AuthUser, role: string) {
    if (actor.role === 'SUPER_ADMIN' && role !== 'SUPER_ADMIN') {
      throw new ForbiddenException('平台账号只能使用管理平台角色');
    }
    if (actor.role === 'TENANT_ADMIN' && role !== 'TENANT_USER') {
      throw new ForbiddenException('租户管理员只能管理普通租户用户');
    }
  }

  private assertLoginCredential(email: string | null, phone: string | null) {
    if (!email && !phone) throw new BadRequestException('邮箱或手机号至少填写一个，作为登录凭证');
  }

  private async assertCredentialAvailable(email: string | null, phone: string | null, excludeId?: string) {
    const candidates: Prisma.UserWhereInput[] = [];
    if (email) candidates.push({ email });
    if (phone) candidates.push({ phone });
    const exists = await this.prisma.user.findFirst({
      where: {
        ...(excludeId ? { id: { not: excludeId } } : {}),
        OR: candidates,
      },
    });
    if (!exists) return;
    if (email && exists.email === email) throw new BadRequestException('该邮箱已被其他账号使用');
    throw new BadRequestException('该手机号已被其他账号使用');
  }

  private async validateScope(tenantId: string, mode: 'ALL' | 'CUSTOM', deviceIds: string[], groupIds: string[]) {
    if (mode === 'ALL') return;
    if (!deviceIds.length && !groupIds.length) throw new BadRequestException('指定设备范围时，请至少选择一个设备分组或设备');
    const [deviceCount, groupCount] = await this.prisma.$transaction([
      this.prisma.device.count({ where: { tenantId, id: { in: deviceIds } } }),
      this.prisma.deviceGroup.count({ where: { tenantId, id: { in: groupIds } } }),
    ]);
    if (deviceCount !== deviceIds.length || groupCount !== groupIds.length) {
      throw new BadRequestException('部分设备或分组不存在，或不属于当前租户');
    }
  }

  private readonly scopeInclude = {
    deviceAccesses: { select: { deviceId: true } },
    deviceGroupAccesses: { select: { groupId: true } },
  } as const;

  private toPublicUser(user: UserWithScope) {
    const { password: _password, deviceAccesses = [], deviceGroupAccesses = [], ...publicUser } = user;
    return { ...publicUser, deviceIds: deviceAccesses.map(item => item.deviceId), groupIds: deviceGroupAccesses.map(item => item.groupId) };
  }
}

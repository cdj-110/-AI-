import { BadRequestException, ConflictException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { AssignDeviceGroupDto, SaveDeviceGroupDto } from './dto/device-group.dto';
import { deviceAccessWhere } from '../common/user-permissions';

@Injectable()
export class DeviceGroupsService {
  constructor(private readonly prisma: PrismaService) {}

  private tenant(actor: AuthUser, write = false) {
    const canWrite = actor.role === 'TENANT_ADMIN'
      || (actor.role === 'TENANT_USER' && actor.permissions?.includes('DEVICE_GROUP_MANAGE'));
    if (!actor.tenantId || !['TENANT_ADMIN', 'TENANT_USER'].includes(actor.role) || (write && !canWrite)) {
      throw new ForbiddenException('无权管理当前租户的设备分组');
    }
    return actor.tenantId;
  }

  async list(actor: AuthUser) {
    const tenantId = this.tenant(actor);
    if (actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM') {
      const [devices, assignedGroups] = await this.prisma.$transaction([
        this.prisma.device.findMany({ where: deviceAccessWhere(actor), select: { id: true, groupMemberships: { select: { groupId: true } } } }),
        this.prisma.userDeviceGroupAccess.findMany({ where: { userId: actor.sub }, select: { groupId: true } }),
      ]);
      const counts = new Map<string, number>();
      let ungroupedDevices = 0;
      for (const device of devices) {
        if (!device.groupMemberships.length) ungroupedDevices++;
        for (const membership of device.groupMemberships) {
          counts.set(membership.groupId, (counts.get(membership.groupId) ?? 0) + 1);
        }
      }
      const groupIds = [...new Set([...assignedGroups.map(item => item.groupId), ...counts.keys()])];
      const groups = groupIds.length ? await this.prisma.deviceGroup.findMany({ where: { tenantId, id: { in: groupIds } }, orderBy: [{ createdAt: 'asc' }, { id: 'asc' }] }) : [];
      return { items: groups.map(group => ({ ...group, _count: { devices: counts.get(group.id) ?? 0 } })), totalDevices: devices.length, ungroupedDevices };
    }
    const [groups, totalDevices, ungroupedDevices] = await this.prisma.$transaction([
      this.prisma.deviceGroup.findMany({ where: { tenantId }, orderBy: [{ createdAt: 'asc' }, { id: 'asc' }], include: { _count: { select: { deviceMemberships: true } } } }),
      this.prisma.device.count({ where: { tenantId } }),
      this.prisma.device.count({ where: { tenantId, groupMemberships: { none: {} } } }),
    ], { isolationLevel: Prisma.TransactionIsolationLevel.RepeatableRead });
    const items = groups.map(({ _count, ...group }) => ({ ...group, _count: { devices: _count.deviceMemberships } }));
    return { items, totalDevices, ungroupedDevices };
  }

  private async change<T>(work: (tx: Prisma.TransactionClient) => Promise<T>): Promise<T> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await this.prisma.$transaction(work, { isolationLevel: Prisma.TransactionIsolationLevel.Serializable });
      } catch (error) {
        if (error instanceof Prisma.PrismaClientKnownRequestError) {
          if (error.code === 'P2034' && attempt < 2) continue;
          if (error.code === 'P2002') throw new ConflictException('该租户已存在同名分组');
          if (['P2003', 'P2025', 'P2034'].includes(error.code)) throw new ConflictException('设备或分组已发生变化，请刷新后重试');
        }
        throw error;
      }
    }
  }

  private async group(tx: Prisma.TransactionClient, tenantId: string, id: string, actor: AuthUser) {
    const group = await tx.deviceGroup.findFirst({
      where: {
        id,
        tenantId,
        ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM'
          ? { userAccesses: { some: { userId: actor.sub } } }
          : {}),
      },
    });
    if (!group) throw new NotFoundException('分组不存在或无权访问');
    return group;
  }

  create(actor: AuthUser, dto: SaveDeviceGroupDto) {
    const tenantId = this.tenant(actor, true);
    return this.change(async tx => {
      const group = await tx.deviceGroup.create({ data: { tenantId, name: dto.name } });
      if (actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM') {
        await tx.userDeviceGroupAccess.create({ data: { userId: actor.sub, groupId: group.id } });
      }
      return group;
    });
  }

  rename(actor: AuthUser, id: string, dto: SaveDeviceGroupDto) {
    const tenantId = this.tenant(actor, true);
    return this.change(async tx => {
      await this.group(tx, tenantId, id, actor);
      return tx.deviceGroup.update({ where: { id }, data: { name: dto.name } });
    });
  }

  remove(actor: AuthUser, id: string) {
    const tenantId = this.tenant(actor, true);
    return this.change(async tx => {
      const group = await this.group(tx, tenantId, id, actor);
      const removedMemberships = await tx.deviceGroupMembership.count({ where: { tenantId, groupId: id } });
      await tx.deviceGroup.delete({ where: { id } });
      return { ...group, removedMemberships };
    });
  }

  assign(actor: AuthUser, dto: AssignDeviceGroupDto) {
    const tenantId = this.tenant(actor, true);
    return this.change(async tx => {
      if (dto.mode !== 'REPLACE' && !dto.groupIds.length) {
        throw new BadRequestException(dto.mode === 'ADD' ? '请选择要加入的分组' : '请选择要移出的分组');
      }
      const groups = dto.groupIds.length
        ? await tx.deviceGroup.findMany({
            where: {
              tenantId,
              id: { in: dto.groupIds },
              ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM'
                ? { userAccesses: { some: { userId: actor.sub } } }
                : {}),
            },
            select: { id: true, name: true },
          })
        : [];
      if (groups.length !== dto.groupIds.length) throw new NotFoundException('部分分组不存在或无权访问');
      const where: Prisma.DeviceWhereInput = {
        AND: [
          { tenantId, id: { in: dto.deviceIds } },
          ...(actor.role === 'TENANT_USER' && actor.deviceScopeMode === 'CUSTOM' ? [deviceAccessWhere(actor)] : []),
        ],
      };
      const count = await tx.device.count({ where });
      if (count !== dto.deviceIds.length) throw new BadRequestException('部分设备不存在或不属于当前租户，未调整任何设备');
      const membershipData = dto.deviceIds.flatMap(deviceId => dto.groupIds.map(groupId => ({ deviceId, groupId, tenantId })));
      let changed = 0;
      if (dto.mode === 'REPLACE') {
        const removed = await tx.deviceGroupMembership.deleteMany({ where: { tenantId, deviceId: { in: dto.deviceIds } } });
        const added = membershipData.length ? await tx.deviceGroupMembership.createMany({ data: membershipData, skipDuplicates: true }) : { count: 0 };
        changed = removed.count + added.count;
      } else if (dto.mode === 'ADD') {
        changed = (await tx.deviceGroupMembership.createMany({ data: membershipData, skipDuplicates: true })).count;
      } else {
        changed = (await tx.deviceGroupMembership.deleteMany({ where: { tenantId, deviceId: { in: dto.deviceIds }, groupId: { in: dto.groupIds } } })).count;
      }
      return { count, changed, mode: dto.mode, groupIds: dto.groupIds, groupNames: groups.map(group => group.name) };
    });
  }
}

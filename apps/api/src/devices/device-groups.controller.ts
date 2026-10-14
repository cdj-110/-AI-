import { Body, Controller, Delete, Get, Param, ParseUUIDPipe, Patch, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { DeviceGroupsService } from './device-groups.service';
import { Permissions } from '../common/decorators/permissions.decorator';
import { PermissionsGuard } from '../common/guards/permissions.guard';
import { AssignDeviceGroupDto, SaveDeviceGroupDto } from './dto/device-group.dto';

@Controller('device-groups')
@UseGuards(JwtAuthGuard, RolesGuard, PermissionsGuard)
@Permissions('DEVICE_VIEW')
export class DeviceGroupsController {
  constructor(private readonly groups: DeviceGroupsService, private readonly logs: OperationLogsService) {}

  private record(actor: AuthUser, request: Request, action: string, targetId: string | undefined, targetName: string, detail?: unknown) {
    return this.logs.record(actor, { module: '设备分组', action, targetType: 'DeviceGroup', targetId, targetName, detail, ip: request.ip, userAgent: request.headers['user-agent'] });
  }

  @Get()
  @Roles('TENANT_ADMIN', 'TENANT_USER')
  list(@CurrentUser() actor: AuthUser) { return this.groups.list(actor); }

  @Post()
  @Roles('TENANT_ADMIN', 'TENANT_USER')
  @Permissions('DEVICE_GROUP_MANAGE')
  async create(@CurrentUser() actor: AuthUser, @Body() dto: SaveDeviceGroupDto, @Req() request: Request) {
    const group = await this.groups.create(actor, dto);
    await this.record(actor, request, '新建分组', group.id, group.name);
    return group;
  }

  @Patch('membership')
  @Roles('TENANT_ADMIN', 'TENANT_USER')
  @Permissions('DEVICE_GROUP_MANAGE')
  async assign(@CurrentUser() actor: AuthUser, @Body() dto: AssignDeviceGroupDto, @Req() request: Request) {
    const result = await this.groups.assign(actor, dto);
    await this.record(actor, request, '调整设备分组', undefined, result.groupNames.join('、') || '清空分组', { deviceIds: dto.deviceIds, groupIds: dto.groupIds, mode: dto.mode, count: result.count, changed: result.changed });
    return result;
  }

  @Patch(':id')
  @Roles('TENANT_ADMIN', 'TENANT_USER')
  @Permissions('DEVICE_GROUP_MANAGE')
  async rename(@CurrentUser() actor: AuthUser, @Param('id', ParseUUIDPipe) id: string, @Body() dto: SaveDeviceGroupDto, @Req() request: Request) {
    const group = await this.groups.rename(actor, id, dto);
    await this.record(actor, request, '重命名分组', group.id, group.name);
    return group;
  }

  @Delete(':id')
  @Roles('TENANT_ADMIN', 'TENANT_USER')
  @Permissions('DEVICE_GROUP_MANAGE')
  async remove(@CurrentUser() actor: AuthUser, @Param('id', ParseUUIDPipe) id: string, @Req() request: Request) {
    const result = await this.groups.remove(actor, id);
    await this.record(actor, request, '删除分组', id, result.name, { removedMemberships: result.removedMemberships });
    return result;
  }
}

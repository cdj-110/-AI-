import { Body, Controller, Delete, Get, Param, Patch, Post, Query, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { UsersService } from './users.service';

@Controller('users')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class UsersController {
  constructor(
    private readonly usersService: UsersService,
    private readonly operationLogsService: OperationLogsService,
  ) {}

  @Get()
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
  ) {
    return this.usersService.findAll(user, Number(page), Number(pageSize), keyword);
  }

  @Post()
  async create(@CurrentUser() user: AuthUser, @Body() dto: CreateUserDto, @Req() request: Request) {
    const created = await this.usersService.create(user, dto);
    await this.operationLogsService.record(user, {
      module: '用户管理',
      action: '创建用户',
      targetType: 'User',
      targetId: created.id,
      targetName: created.username,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { username: dto.username, role: dto.role, tenantId: dto.tenantId },
    });
    return created;
  }

  @Get(':id')
  findOne(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.usersService.findOne(user, id);
  }

  @Patch(':id')
  async update(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: UpdateUserDto, @Req() request: Request) {
    const updated = await this.usersService.update(user, id, dto);
    await this.operationLogsService.record(user, {
      module: '用户管理',
      action: '编辑用户',
      targetType: 'User',
      targetId: id,
      targetName: updated.username,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { ...dto, password: dto.password ? '已修改' : undefined },
    });
    return updated;
  }

  @Delete(':id')
  async remove(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.usersService.remove(user, id);
    await this.operationLogsService.record(user, {
      module: '用户管理',
      action: '删除用户',
      targetType: 'User',
      targetId: id,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    });
    return result;
  }

  private clientIp(request: Request) {
    const forwardedFor = request.headers['x-forwarded-for'];
    const firstForwardedIp = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor?.split(',')[0];
    return (firstForwardedIp || request.headers['x-real-ip'] || request.ip || request.socket.remoteAddress || '').toString().replace('::ffff:', '');
  }
}

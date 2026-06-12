import { Controller, Get, Param, Patch, Query, Req, Sse, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { AlarmsService } from './alarms.service';

@Controller('alarms')
@UseGuards(JwtAuthGuard, RolesGuard)
export class AlarmsController {
  constructor(
    private readonly alarmsService: AlarmsService,
    private readonly operationLogsService: OperationLogsService,
  ) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('status') status = '',
  ) {
    return this.alarmsService.findAll(user, Number(page), Number(pageSize), status);
  }

  @Sse('stream')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  stream(@CurrentUser() user: AuthUser) {
    return this.alarmsService.stream(user);
  }

  @Patch(':id/resolve')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async resolve(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const alarm = await this.alarmsService.resolve(user, id);
    await this.operationLogsService.record(user, {
      module: '告警管理',
      action: '处理告警',
      targetType: 'Alarm',
      targetId: id,
      targetName: alarm.message,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { level: alarm.level, type: alarm.type, deviceId: alarm.deviceId },
    });
    return alarm;
  }

  private clientIp(request: Request) {
    const forwardedFor = request.headers['x-forwarded-for'];
    const firstForwardedIp = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor?.split(',')[0];
    return (firstForwardedIp || request.headers['x-real-ip'] || request.ip || request.socket.remoteAddress || '').toString().replace('::ffff:', '');
  }
}

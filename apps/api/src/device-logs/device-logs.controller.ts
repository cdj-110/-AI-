import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { DeviceLogsService } from './device-logs.service';

@Controller('device-logs')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class DeviceLogsController {
  constructor(private readonly deviceLogsService: DeviceLogsService) {}

  @Get()
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
    @Query('type') type = '',
    @Query('level') level = '',
    @Query('source') source = '',
    @Query('deviceId') deviceId = '',
  ) {
    return this.deviceLogsService.findAll(user, Number(page), Number(pageSize), { keyword, type, level, source, deviceId });
  }
}

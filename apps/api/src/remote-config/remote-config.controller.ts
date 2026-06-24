import { Body, Controller, Get, Param, Post, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateRemoteConfigDto } from './dto/create-remote-config.dto';
import { RemoteConfigService } from './remote-config.service';

@Controller('devices/:deviceId/remote-configs')
@UseGuards(JwtAuthGuard, RolesGuard)
export class RemoteConfigController {
  constructor(private readonly service: RemoteConfigService) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  list(@CurrentUser() user: AuthUser, @Param('deviceId') deviceId: string) {
    return this.service.list(user, deviceId);
  }

  @Get('current')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  current(@CurrentUser() user: AuthUser, @Param('deviceId') deviceId: string) {
    return this.service.current(user, deviceId);
  }

  @Post()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  create(@CurrentUser() user: AuthUser, @Param('deviceId') deviceId: string, @Body() dto: CreateRemoteConfigDto) {
    return this.service.create(user, deviceId, dto);
  }

  @Post(':taskId/retry')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  retry(@CurrentUser() user: AuthUser, @Param('deviceId') deviceId: string, @Param('taskId') taskId: string) {
    return this.service.retry(user, deviceId, taskId);
  }
}

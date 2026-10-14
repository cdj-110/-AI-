import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { EventsService } from './events.service';
import { Permissions } from '../common/decorators/permissions.decorator';
import { PermissionsGuard } from '../common/guards/permissions.guard';

@Controller('events')
@UseGuards(JwtAuthGuard, RolesGuard, PermissionsGuard)
@Permissions('EVENT_VIEW')
export class EventsController {
  constructor(private readonly eventsService: EventsService) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '20',
    @Query('state') state = '',
    @Query('level') level = '',
    @Query('gatewayId') gatewayId = '',
    @Query('deviceId') deviceId = '',
    @Query('keyword') keyword = '',
    @Query('startAt') startAt = '',
    @Query('endAt') endAt = '',
    @Query('tenantId') tenantId = '',
  ) {
    return this.eventsService.findAll(user, {
      page: Number(page), pageSize: Number(pageSize), state, level, gatewayId, deviceId, keyword, startAt, endAt, tenantId,
    });
  }
}

import { Controller, Get, Param, Patch, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { AlarmsService } from './alarms.service';

@Controller('alarms')
@UseGuards(JwtAuthGuard, RolesGuard)
export class AlarmsController {
  constructor(private readonly alarmsService: AlarmsService) {}

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

  @Patch(':id/resolve')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  resolve(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.alarmsService.resolve(user, id);
  }
}

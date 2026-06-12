import { Controller, Get, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { SystemStatusService } from './system-status.service';

@Controller('system-status')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class SystemStatusController {
  constructor(private readonly systemStatusService: SystemStatusService) {}

  @Get()
  status(@CurrentUser() user: AuthUser) {
    return this.systemStatusService.status(user);
  }
}

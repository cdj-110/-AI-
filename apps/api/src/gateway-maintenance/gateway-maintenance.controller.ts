import { Body, Controller, Get, Param, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateMaintenanceTaskDto } from './create-maintenance-task.dto';
import { GatewayMaintenanceService } from './gateway-maintenance.service';

@Controller('gateway-maintenance/gateways/:gatewayId')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN')
export class GatewayMaintenanceController {
  constructor(private readonly maintenance: GatewayMaintenanceService) {}

  @Get('tasks')
  list(@Param('gatewayId') gatewayId: string) { return this.maintenance.list(gatewayId); }

  @Get('status')
  status(@Param('gatewayId') gatewayId: string) { return this.maintenance.status(gatewayId); }

  @Post('tasks')
  create(@Param('gatewayId') gatewayId: string, @Body() dto: CreateMaintenanceTaskDto, @CurrentUser() actor: AuthUser, @Req() request: Request) {
    return this.maintenance.create(gatewayId, dto.action, actor, { ip: request.ip, userAgent: request.headers['user-agent'] });
  }
}

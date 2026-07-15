import { Controller, Get, Param, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { GatewayTopologyService } from './gateway-topology.service';

@Controller('devices/:gatewayId/topology-draft')
@UseGuards(JwtAuthGuard, RolesGuard)
export class GatewayTopologyController {
  constructor(
    private readonly topology: GatewayTopologyService,
    private readonly operationLogs: OperationLogsService,
  ) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findDraft(@CurrentUser() user: AuthUser, @Param('gatewayId') gatewayId: string) {
    return this.topology.findDraft(user, gatewayId);
  }

  @Post('confirm')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async confirm(@CurrentUser() user: AuthUser, @Param('gatewayId') gatewayId: string, @Req() request: Request) {
    const result = await this.topology.confirm(user, gatewayId);
    await this.operationLogs.record(user, {
      module: '网关拓扑',
      action: '确认拓扑草稿',
      targetType: 'GatewayTopologyDraft',
      targetId: gatewayId,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: result,
    });
    return result;
  }

  @Post('ignore')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async ignore(@CurrentUser() user: AuthUser, @Param('gatewayId') gatewayId: string, @Req() request: Request) {
    const result = await this.topology.ignore(user, gatewayId);
    await this.operationLogs.record(user, {
      module: '网关拓扑',
      action: '忽略拓扑草稿',
      targetType: 'GatewayTopologyDraft',
      targetId: result.id,
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

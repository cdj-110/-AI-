import { Controller, Get, Param, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { RemoteAccessService } from './remote-access.service';
import { Permissions } from '../common/decorators/permissions.decorator';
import { PermissionsGuard } from '../common/guards/permissions.guard';

@Controller('remote-access/devices')
@UseGuards(JwtAuthGuard, RolesGuard, PermissionsGuard)
@Permissions('DEVICE_VIEW')
export class RemoteAccessController {
  constructor(private readonly remote: RemoteAccessService, private readonly logs: OperationLogsService) {}

  @Get(':id/status')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  status(@CurrentUser() actor: AuthUser, @Param('id') id: string) { return this.remote.status(actor, id); }

  @Post(':id/sessions')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async create(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const origin = `${request.headers['x-forwarded-proto'] ?? request.protocol}://${request.headers['x-forwarded-host'] ?? request.headers.host}`;
    const result = await this.remote.createSession(actor, id, origin);
    await this.logs.record(actor, { module: '远程访问', action: '创建远程 Web 会话', targetType: 'Device', targetId: id, ip: request.ip, userAgent: request.headers['user-agent'], detail: { sessionId: result.sessionId, expiresAt: result.expiresAt } });
    return result;
  }

  @Post(':id/terminal-sessions')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async createTerminal(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const origin = `${request.headers['x-forwarded-proto'] ?? request.protocol}://${request.headers['x-forwarded-host'] ?? request.headers.host}`;
    const result = await this.remote.createTerminalSession(actor, id, origin);
    await this.logs.record(actor, {
      module: '远程访问', action: '创建远程 SSH 会话', targetType: 'Device', targetId: id,
      ip: request.ip, userAgent: request.headers['user-agent'],
      detail: { sessionId: result.sessionId, expiresAt: result.expiresAt },
    });
    return result;
  }
}

import { Body, Controller, Get, Param, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateDeviceCommandDto } from './create-device-command.dto';
import { DeviceCommandsService } from './device-commands.service';

@Controller('devices/:id/commands')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class DeviceCommandsController {
  constructor(private readonly commands: DeviceCommandsService) {}

  @Get('capabilities')
  capabilities(@CurrentUser() actor: AuthUser, @Param('id') id: string) { return this.commands.capabilities(actor, id); }

  @Get()
  list(@CurrentUser() actor: AuthUser, @Param('id') id: string) { return this.commands.list(actor, id); }

  @Post()
  create(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: CreateDeviceCommandDto, @Req() request: Request) {
    return this.commands.create(actor, id, dto, { ip: request.ip, userAgent: request.headers['user-agent'] });
  }
}

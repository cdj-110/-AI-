import { Body, Controller, Get, Param, Post, Query, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { BindFactoryGatewayDto } from './dto/bind-factory-gateway.dto';
import { RegisterFactoryGatewayDto } from './dto/register-factory-gateway.dto';
import { FactoryGatewaysService } from './factory-gateways.service';

@Controller('factory-gateways')
@UseGuards(JwtAuthGuard, RolesGuard)
export class FactoryGatewaysController {
  constructor(
    private readonly factoryGatewaysService: FactoryGatewaysService,
    private readonly operationLogsService: OperationLogsService,
  ) {}

  @Get()
  @Roles('SUPER_ADMIN')
  findAll(@Query('page') page = '1', @Query('pageSize') pageSize = '10', @Query('keyword') keyword = '') {
    return this.factoryGatewaysService.findAll(Number(page), Number(pageSize), keyword);
  }

  @Post()
  @Roles('SUPER_ADMIN')
  async register(@CurrentUser() user: AuthUser, @Body() dto: RegisterFactoryGatewayDto, @Req() request: Request) {
    const result = await this.factoryGatewaysService.register(dto);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '登记出厂网关',
      targetType: 'FactoryGateway',
      targetId: result.id,
      targetName: result.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { hardwareId: dto.hardwareId, batchNo: dto.batchNo },
    });
    return result;
  }

  @Post('bind')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async bind(@CurrentUser() user: AuthUser, @Body() dto: BindFactoryGatewayDto, @Req() request: Request) {
    const result = await this.factoryGatewaysService.bind(user, dto);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '绑定网关',
      targetType: 'Device',
      targetId: result.device.id,
      targetName: result.device.name,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: dto.sn },
    });
    return result;
  }

  @Post(':id/bind-code/reset')
  @Roles('SUPER_ADMIN')
  async resetBindCode(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.resetBindCode(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '重置绑定码',
      targetType: 'FactoryGateway',
      targetId: result.id,
      targetName: result.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: result.sn },
    });
    return result;
  }

  @Post(':id/activation/reset')
  @Roles('SUPER_ADMIN')
  async resetActivation(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.resetActivation(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '重新生成激活文件',
      targetType: 'FactoryGateway',
      targetId: result.id,
      targetName: result.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: result.sn, hardwareId: result.hardwareId },
    });
    return result;
  }

  private clientIp(request: Request) {
    const forwardedFor = request.headers['x-forwarded-for'];
    const firstForwardedIp = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor?.split(',')[0];
    return (firstForwardedIp || request.headers['x-real-ip'] || request.ip || request.socket.remoteAddress || '').toString().replace('::ffff:', '');
  }
}

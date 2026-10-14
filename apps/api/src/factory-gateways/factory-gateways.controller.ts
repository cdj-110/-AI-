import { Body, Controller, Delete, Get, Param, Post, Query, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { BindFactoryGatewayDto } from './dto/bind-factory-gateway.dto';
import { RegisterFactoryGatewayDto } from './dto/register-factory-gateway.dto';
import { RecordNfcWriteDto } from './dto/record-nfc-write.dto';
import { FactoryGatewaysService } from './factory-gateways.service';
import { BackfillGatewayIdentityDto } from './dto/backfill-gateway-identity.dto';

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

  @Get('nfc-write-history')
  @Roles('SUPER_ADMIN')
  nfcWriteHistory(@Query('page') page = '1', @Query('pageSize') pageSize = '50') {
    return this.operationLogsService.findNfcWriteHistory(Number(page), Number(pageSize));
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
      detail: { hardwareId: dto.hardwareId, batchNo: result.batchNo, model: result.model },
    });
    return result;
  }

  @Post('bind')
  @Roles('TENANT_ADMIN')
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

  @Post('bind/verify')
  @Roles('TENANT_ADMIN')
  verifyBind(@CurrentUser() user: AuthUser, @Body() dto: BindFactoryGatewayDto) {
    return this.factoryGatewaysService.verifyBind(user, dto);
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

  @Get(':id/bind-credential')
  @Roles('SUPER_ADMIN')
  bindCredential(@Param('id') id: string) {
    return this.factoryGatewaysService.getBindCredential(id);
  }

  @Post(':id/nfc-credential')
  @Roles('SUPER_ADMIN')
  async prepareNfcCredential(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.prepareNfcCredential(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '读取或生成 NFC 绑定凭证',
      targetType: 'FactoryGateway',
      targetId: result.id,
      targetName: result.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: result.sn, nfcIssuedAt: result.nfcIssuedAt },
    });
    return result;
  }

  @Post(':id/nfc-credential/reset')
  @Roles('SUPER_ADMIN')
  async resetNfcCredential(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.resetNfcCredential(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关',
      action: '重置 NFC 绑定凭证',
      targetType: 'FactoryGateway',
      targetId: result.id,
      targetName: result.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: result.sn, nfcIssuedAt: result.nfcIssuedAt },
    });
    return result;
  }

  @Post(':id/nfc-write-result')
  @Roles('SUPER_ADMIN')
  async recordNfcWriteResult(
    @CurrentUser() user: AuthUser,
    @Param('id') id: string,
    @Body() dto: RecordNfcWriteDto,
    @Req() request: Request,
  ) {
    await this.operationLogsService.record(user, {
      module: 'NFC 标签',
      action: dto.success ? 'NFC 写入成功' : 'NFC 写入失败',
      targetType: 'FactoryGateway',
      targetId: id,
      targetName: dto.sn,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { sn: dto.sn, success: dto.success, message: dto.message || null },
    });
    return { ok: true };
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

  @Post(':id/chip-identity')
  @Roles('SUPER_ADMIN')
  async backfillIdentity(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: BackfillGatewayIdentityDto, @Req() request: Request) {
    const result = await this.factoryGatewaysService.backfillIdentity(id, dto);
    await this.operationLogsService.record(user, {
      module: '出厂网关', action: '补录设备授权身份', targetType: 'FactoryGateway',
      targetId: result.id, targetName: result.sn, ip: this.clientIp(request), userAgent: request.headers['user-agent'],
      detail: { hardwareId: result.hardwareId, chipUsid: result.chipUsid, credentialRecovered: Boolean(dto.deviceSecret) },
    });
    return result;
  }

  @Post(':id/reclaim')
  @Roles('SUPER_ADMIN')
  async reclaim(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.reclaim(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关', action: '回收已绑定网关', targetType: 'FactoryGateway',
      targetId: result.id, targetName: result.sn, ip: this.clientIp(request), userAgent: request.headers['user-agent'],
      detail: { sn: result.sn, hardwareId: result.hardwareId, releasedDevice: result.device, releasedChildrenCount: result.releasedChildrenCount },
    });
    return { message: result.releasedChildrenCount > 0
      ? `网关已回收，${result.releasedChildrenCount} 个子设备已解除关联并标记离线，档案和数据保留在原租户`
      : '网关已解除租户绑定并恢复为待绑定', ...result };
  }

  @Delete(':id')
  @Roles('SUPER_ADMIN')
  async remove(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.factoryGatewaysService.removeUnbound(id);
    await this.operationLogsService.record(user, {
      module: '出厂网关', action: '移除未绑定网关', targetType: 'FactoryGateway',
      targetId: result.id, targetName: result.sn, ip: this.clientIp(request), userAgent: request.headers['user-agent'],
      detail: { sn: result.sn, hardwareId: result.hardwareId, archived: true },
    });
    return { message: '出厂网关已移除，HardwareId 与 SN 档案已保留', ...result };
  }

  private clientIp(request: Request) {
    const forwardedFor = request.headers['x-forwarded-for'];
    const firstForwardedIp = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor?.split(',')[0];
    return (firstForwardedIp || request.headers['x-real-ip'] || request.ip || request.socket.remoteAddress || '').toString().replace('::ffff:', '');
  }
}

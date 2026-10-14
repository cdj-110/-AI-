import { Body, Controller, Delete, Get, Param, Post, Query, Req, Res, UseGuards } from '@nestjs/common';
import { Request, Response } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { OtaService } from './ota.service';

@Controller('ota')
export class OtaController {
  constructor(private readonly ota: OtaService, private readonly logs: OperationLogsService) {}

  @Get('readiness')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  readiness() { return this.ota.readiness(); }

  @Get('packages')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  packages() { return this.ota.listPackages(); }

  @Post('packages')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  async upload(@Req() request: Request, @CurrentUser() actor: AuthUser) {
    const pack = await this.ota.upload(request, actor);
    await this.audit(actor, request, '上传升级包', pack.id, `${pack.model} ${pack.version}`, pack);
    return pack;
  }

  @Delete('packages/:packageId')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  async removePackage(@Param('packageId') packageId: string, @Req() request: Request, @CurrentUser() actor: AuthUser) {
    const pack = await this.ota.purgePackage(packageId);
    await this.audit(actor, request, '清理升级包文件', pack.id, `${pack.model} ${pack.version}`, pack);
    return pack;
  }

  @Get('gateways/:gatewayId/tasks')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  tasks(@Param('gatewayId') gatewayId: string) { return this.ota.listTasks(gatewayId); }

  @Post('gateways/:gatewayId/tasks')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  create(@Param('gatewayId') gatewayId: string, @Body('packageId') packageId: string, @CurrentUser() actor: AuthUser) {
    return this.ota.createTask(gatewayId, packageId, actor);
  }

  @Get('download/:packageId')
  download(@Param('packageId') packageId: string, @Query('taskId') taskId: string, @Query('exp') exp: string, @Query('sig') sig: string, @Res() response: Response) {
    return this.ota.download(packageId, taskId, exp, sig, response);
  }

  private audit(actor: AuthUser, request: Request, action: string, id: string, name: string, detail: object) {
    return this.logs.record(actor, {
      module: '升级包管理', action, targetType: 'OtaPackage', targetId: id, targetName: name,
      ip: request.ip || request.socket.remoteAddress, userAgent: request.headers['user-agent'], detail,
    });
  }
}

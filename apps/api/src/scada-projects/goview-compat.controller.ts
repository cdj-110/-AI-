import { Body, Controller, Get, Post, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Permissions } from '../common/decorators/permissions.decorator';
import { RawResponse } from '../common/decorators/raw-response.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { PermissionsGuard } from '../common/guards/permissions.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { ScadaProjectsService } from './scada-projects.service';
import { DevicesService } from '../devices/devices.service';

type GoViewConfigBody = {
  config_id?: string;
  main_data?: Record<string, unknown>;
  thumbnail?: string;
};

@Controller()
@RawResponse()
@UseGuards(JwtAuthGuard, RolesGuard, PermissionsGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
@Permissions('SCADA_VIEW')
export class GoViewCompatController {
  constructor(
    private readonly projects: ScadaProjectsService,
    private readonly devicesService: DevicesService,
  ) {}

  @Get('conf/config/datas_unlocked')
  loadUnlocked(@CurrentUser() actor: AuthUser, @Query('config_id') configId = '') {
    return this.projects.goViewConfig(actor, configId);
  }

  @Get('conf/config/datas')
  load(@CurrentUser() actor: AuthUser, @Query('config_id') configId = '') {
    return this.projects.goViewConfig(actor, configId);
  }

  @Post('conf/config')
  async save(@CurrentUser() actor: AuthUser, @Body() body: GoViewConfigBody) {
    await this.projects.saveGoViewConfig(actor, body.config_id || '', body.main_data, body.thumbnail);
    return { success: true, message: '组态保存成功' };
  }

  @Post('configuration/point-bindings')
  async saveBindings(@CurrentUser() actor: AuthUser, @Body() body: { config_id?: string }) {
    await this.projects.findOne(actor, body.config_id || '');
    return { code: 200, msg: 'ok' };
  }

  @Post('deviceinfo/deviceinfo')
  async devices(@CurrentUser() actor: AuthUser) {
    return { code: 200, msg: 'ok', data: { data: await this.projects.goViewDevices(actor) } };
  }

  @Post('deviceinfo/device_detail_function')
  async metrics(@CurrentUser() actor: AuthUser, @Body() body: { model_id?: string }) {
    const metrics = await this.projects.goViewMetrics(actor, body.model_id || '');
    return { code: 200, msg: 'ok', data: { data: metrics } };
  }

  @Post('conf/influx/analysis/query/v3_2')
  historyTable(
    @CurrentUser() actor: AuthUser,
    @Body() body: {
      start?: string;
      end?: string;
      pairs?: Array<{ value1?: string; value2?: string }>;
      page?: number;
      pageSize?: number;
      keyword?: string;
      chart?: boolean;
    },
  ) {
    return this.devicesService.telemetryHistoryTable(
      actor,
      body.pairs ?? [],
      body.start,
      body.end,
      body.page,
      body.pageSize,
      body.keyword,
      body.chart === true,
    );
  }

  @Get('conf/history-retention')
  historyWindow(@CurrentUser() actor: AuthUser) { return this.devicesService.historyWindow(actor); }

  @Post('conf/influx/analysis/export')
  exportHistory(@CurrentUser() actor: AuthUser, @Body() body: { pairs?: Array<{ value1?: string; value2?: string }>; start?: string; end?: string; keyword?: string }) {
    return this.devicesService.exportHistory(actor, body.pairs ?? [], body.start, body.end, body.keyword);
  }
}

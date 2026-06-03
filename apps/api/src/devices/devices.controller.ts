import { Body, Controller, Delete, Get, Param, Patch, Post, Query, Sse, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateDeviceDto } from './dto/create-device.dto';
import { CreateDeviceAlarmRuleDto } from './dto/create-device-alarm-rule.dto';
import { ImportDeviceMetricsDto } from './dto/import-device-metrics.dto';
import { ReportDeviceStatusDto } from './dto/report-device-status.dto';
import { UpdateDeviceMetricDto } from './dto/update-device-metric.dto';
import { UpdateDeviceDto } from './dto/update-device.dto';
import { DevicesService } from './devices.service';

@Controller('devices')
@UseGuards(JwtAuthGuard, RolesGuard)
export class DevicesController {
  constructor(private readonly devicesService: DevicesService) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
  ) {
    return this.devicesService.findAll(user, Number(page), Number(pageSize), keyword);
  }

  @Post()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateDeviceDto) {
    return this.devicesService.create(user, dto);
  }

  @Get('model-templates')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  modelTemplates(@CurrentUser() user: AuthUser) {
    return this.devicesService.modelTemplates(user);
  }

  @Sse('status/stream')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  statusStream(@CurrentUser() user: AuthUser) {
    return this.devicesService.statusStream(user);
  }

  @Get(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findOne(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.findOne(user, id);
  }

  @Patch(':id/runtime-status')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  reportStatus(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: ReportDeviceStatusDto) {
    return this.devicesService.reportStatus(user, id, dto);
  }

  @Get(':id/telemetry')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  telemetry(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.telemetry(user, id);
  }

  @Sse(':id/telemetry/stream')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  telemetryStream(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.telemetryStream(user, id);
  }

  @Get(':id/metrics')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  metrics(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.metrics(user, id);
  }

  @Patch(':id/metrics/:metricId')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  updateMetric(@CurrentUser() user: AuthUser, @Param('id') id: string, @Param('metricId') metricId: string, @Body() dto: UpdateDeviceMetricDto) {
    return this.devicesService.updateMetric(user, id, metricId, dto);
  }

  @Post(':id/metrics/import')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  importMetrics(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: ImportDeviceMetricsDto) {
    return this.devicesService.importMetrics(user, id, dto);
  }

  @Get(':id/alarm-rules')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  alarmRules(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.alarmRules(user, id);
  }

  @Post(':id/alarm-rules')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  createAlarmRule(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: CreateDeviceAlarmRuleDto) {
    return this.devicesService.createAlarmRule(user, id, dto);
  }

  @Delete(':id/alarm-rules/:ruleId')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  removeAlarmRule(@CurrentUser() user: AuthUser, @Param('id') id: string, @Param('ruleId') ruleId: string) {
    return this.devicesService.removeAlarmRule(user, id, ruleId);
  }

  @Patch(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  update(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: UpdateDeviceDto) {
    return this.devicesService.update(user, id, dto);
  }

  @Delete(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  remove(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.remove(user, id);
  }
}

import { Body, Controller, Delete, Get, Param, Patch, Post, Query, Req, Sse, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { CreateDeviceDto } from './dto/create-device.dto';
import { CreateDeviceAlarmRuleDto } from './dto/create-device-alarm-rule.dto';
import { ImportDeviceMetricsDto } from './dto/import-device-metrics.dto';
import { ReportDeviceStatusDto } from './dto/report-device-status.dto';
import { UpdateDeviceAlarmRuleDto } from './dto/update-device-alarm-rule.dto';
import { UpdateDeviceMetricDto } from './dto/update-device-metric.dto';
import { UpdateDeviceDto } from './dto/update-device.dto';
import { DevicesService } from './devices.service';

@Controller('devices')
@UseGuards(JwtAuthGuard, RolesGuard)
export class DevicesController {
  constructor(
    private readonly devicesService: DevicesService,
    private readonly operationLogsService: OperationLogsService,
  ) {}

  // 设备列表用于后台管理页，状态会由前端全局状态流做实时修正。
  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
    @Query('deviceType') deviceType = '',
  ) {
    return this.devicesService.findAll(user, Number(page), Number(pageSize), keyword, deviceType);
  }

  // 创建设备时可以选择物模型模板，后端会复制模板字段到设备自己的物模型中。
  @Post()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async create(@CurrentUser() user: AuthUser, @Body() dto: CreateDeviceDto, @Req() request: Request) {
    const device = await this.devicesService.create(user, dto);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '创建设备',
      targetType: 'Device',
      targetId: device?.id,
      targetName: device?.name ?? dto.name,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { deviceKey: dto.deviceKey, deviceType: dto.deviceType ?? 'DIRECT' },
    });
    return device;
  }

  // 旧版“从已有设备导入物模型”的候选设备列表，保留用于兼容现有配置入口。
  @Get('model-templates')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  modelTemplates(@CurrentUser() user: AuthUser) {
    return this.devicesService.modelTemplates(user);
  }

  // 网关子设备创建/编辑时使用，只返回当前租户可访问的网关设备。
  @Get('gateways')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  gateways(@CurrentUser() user: AuthUser) {
    return this.devicesService.gateways(user);
  }

  // 设备在线/离线状态的统一 SSE 事件流，前端列表、详情、通知都消费这个通道。
  @Sse('status/stream')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  statusStream(@CurrentUser() user: AuthUser) {
    return this.devicesService.statusStream(user);
  }

  // 设备详情接口会带出网关/子设备关系和 MQTT 连接信息。
  @Get(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findOne(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.findOne(user, id);
  }

  @Get(':id/credentials')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  credentials(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.credentials(user, id);
  }

  @Post(':id/credentials/rotate')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async rotateCredentials(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.devicesService.rotateCredentials(user, id);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '重置MQTT密码',
      targetType: 'Device',
      targetId: id,
      targetName: result.clientId,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    });
    return result;
  }

  @Patch(':id/runtime-status')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  reportStatus(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: ReportDeviceStatusDto) {
    return this.devicesService.reportStatus(user, id, dto);
  }

  // 历史遥测原始数据，仅取最近 20 条用于“最近上报”列表和初始页面数据。
  @Get(':id/telemetry')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  telemetry(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.telemetry(user, id);
  }

  // 趋势聚合接口：按前端选择的时间范围在 TimescaleDB 中做 time_bucket 聚合。
  @Get(':id/telemetry/trend')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  telemetryTrend(
    @CurrentUser() user: AuthUser,
    @Param('id') id: string,
    @Query('metric') metric: string,
    @Query('range') range = '15m',
  ) {
    return this.devicesService.telemetryTrend(user, id, metric, range);
  }

  // 单设备遥测实时流，详情页实时指标卡片从这里接收新数据。
  @Sse(':id/telemetry/stream')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  telemetryStream(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.telemetryStream(user, id);
  }

  // 设备物模型字段配置，包含显示、忽略、排序、小数位等 UI 展示配置。
  @Get(':id/metrics')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  metrics(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.metrics(user, id);
  }

  @Patch(':id/metrics/:metricId')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async updateMetric(@CurrentUser() user: AuthUser, @Param('id') id: string, @Param('metricId') metricId: string, @Body() dto: UpdateDeviceMetricDto, @Req() request: Request) {
    const metric = await this.devicesService.updateMetric(user, id, metricId, dto);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '修改物模型字段',
      targetType: 'DeviceMetric',
      targetId: metric.id,
      targetName: metric.identifier,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: dto,
    });
    return metric;
  }

  @Post(':id/metrics/import')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async importMetrics(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: ImportDeviceMetricsDto, @Req() request: Request) {
    const result = await this.devicesService.importMetrics(user, id, dto);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '导入物模型',
      targetType: 'Device',
      targetId: id,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: { ...dto, ...result },
    });
    return result;
  }

  @Get(':id/alarm-rules')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  alarmRules(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.devicesService.alarmRules(user, id);
  }

  @Post(':id/alarm-rules')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async createAlarmRule(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: CreateDeviceAlarmRuleDto, @Req() request: Request) {
    const rule = await this.devicesService.createAlarmRule(user, id, dto);
    await this.operationLogsService.record(user, {
      module: '告警规则',
      action: '保存告警规则',
      targetType: 'DeviceAlarmRule',
      targetId: rule.id,
      targetName: `${rule.identifier} ${rule.operator} ${rule.threshold}`,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: dto,
    });
    return rule;
  }

  @Patch(':id/alarm-rules/:ruleId')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async updateAlarmRule(@CurrentUser() user: AuthUser, @Param('id') id: string, @Param('ruleId') ruleId: string, @Body() dto: UpdateDeviceAlarmRuleDto, @Req() request: Request) {
    const rule = await this.devicesService.updateAlarmRule(user, id, ruleId, dto);
    await this.operationLogsService.record(user, {
      module: '告警规则',
      action: '修改告警规则',
      targetType: 'DeviceAlarmRule',
      targetId: rule.id,
      targetName: `${rule.identifier} ${rule.operator} ${rule.threshold}`,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: dto,
    });
    return rule;
  }

  @Delete(':id/alarm-rules/:ruleId')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async removeAlarmRule(@CurrentUser() user: AuthUser, @Param('id') id: string, @Param('ruleId') ruleId: string, @Req() request: Request) {
    const result = await this.devicesService.removeAlarmRule(user, id, ruleId);
    await this.operationLogsService.record(user, {
      module: '告警规则',
      action: '删除告警规则',
      targetType: 'DeviceAlarmRule',
      targetId: ruleId,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    });
    return result;
  }

  @Patch(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async update(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: UpdateDeviceDto, @Req() request: Request) {
    const device = await this.devicesService.update(user, id, dto);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '编辑设备',
      targetType: 'Device',
      targetId: id,
      targetName: device.name,
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
      detail: dto,
    });
    return device;
  }

  @Delete(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  async remove(@CurrentUser() user: AuthUser, @Param('id') id: string, @Req() request: Request) {
    const result = await this.devicesService.remove(user, id);
    await this.operationLogsService.record(user, {
      module: '设备管理',
      action: '删除设备',
      targetType: 'Device',
      targetId: id,
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

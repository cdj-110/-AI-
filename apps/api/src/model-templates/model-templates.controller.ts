import { Body, Controller, Delete, Get, Param, Patch, Post, Put, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateTemplateFromDeviceDto } from './dto/create-template-from-device.dto';
import { CreateModelTemplateDto } from './dto/create-model-template.dto';
import { SaveModelTemplateMetricsDto, UpdateModelTemplateDto } from './dto/update-model-template.dto';
import { ModelTemplatesService } from './model-templates.service';

@Controller('model-templates')
@UseGuards(JwtAuthGuard, RolesGuard)
export class ModelTemplatesController {
  constructor(private readonly modelTemplatesService: ModelTemplatesService) {}

  @Get()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findAll(@CurrentUser() user: AuthUser, @Query('keyword') keyword = '') {
    return this.modelTemplatesService.findAll(user, keyword);
  }

  @Get('options')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  options(@CurrentUser() user: AuthUser) {
    return this.modelTemplatesService.options(user);
  }

  @Post('from-device')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  createFromDevice(@CurrentUser() user: AuthUser, @Body() dto: CreateTemplateFromDeviceDto) {
    return this.modelTemplatesService.createFromDevice(user, dto);
  }

  @Post(':id/duplicate')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  duplicate(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.modelTemplatesService.duplicate(user, id);
  }

  @Get(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
  findOne(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.modelTemplatesService.findOne(user, id);
  }

  @Post()
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateModelTemplateDto) {
    return this.modelTemplatesService.create(user, dto);
  }

  @Patch(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  update(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: UpdateModelTemplateDto) {
    return this.modelTemplatesService.update(user, id, dto);
  }

  @Put(':id/metrics')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  saveMetrics(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: SaveModelTemplateMetricsDto) {
    return this.modelTemplatesService.saveMetrics(user, id, dto.metrics ?? []);
  }

  @Delete(':id')
  @Roles('SUPER_ADMIN', 'TENANT_ADMIN')
  remove(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.modelTemplatesService.remove(user, id);
  }
}

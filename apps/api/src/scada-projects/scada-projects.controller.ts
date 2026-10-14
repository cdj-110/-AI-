import { Body, Controller, Delete, Get, Param, Patch, Post, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Permissions } from '../common/decorators/permissions.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { PermissionsGuard } from '../common/guards/permissions.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateScadaProjectDto } from './dto/create-scada-project.dto';
import { UpdateScadaProjectDto } from './dto/update-scada-project.dto';
import { ScadaProjectsService } from './scada-projects.service';

@Controller('scada-projects')
@UseGuards(JwtAuthGuard, RolesGuard, PermissionsGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER')
@Permissions('SCADA_VIEW')
export class ScadaProjectsController {
  constructor(private readonly projects: ScadaProjectsService) {}

  @Get()
  list(@CurrentUser() actor: AuthUser, @Query('keyword') keyword = '') {
    return this.projects.list(actor, keyword);
  }

  @Get(':id')
  findOne(@CurrentUser() actor: AuthUser, @Param('id') id: string) {
    return this.projects.findOne(actor, id);
  }

  @Post()
  create(@CurrentUser() actor: AuthUser, @Body() dto: CreateScadaProjectDto) {
    return this.projects.create(actor, dto);
  }

  @Post(':id/duplicate')
  duplicate(@CurrentUser() actor: AuthUser, @Param('id') id: string) {
    return this.projects.duplicate(actor, id);
  }

  @Patch(':id')
  update(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: UpdateScadaProjectDto) {
    return this.projects.update(actor, id, dto);
  }

  @Delete(':id')
  remove(@CurrentUser() actor: AuthUser, @Param('id') id: string) {
    return this.projects.remove(actor, id);
  }
}

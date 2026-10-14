import { Body, Controller, Delete, Get, Param, Patch, Post, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { SaveDeviceLinkageDto, EnableDeviceLinkageDto } from './device-linkage.dto';
import { DeviceLinkageService } from './device-linkage.service';

@Controller('device-linkage')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('TENANT_ADMIN')
export class DeviceLinkageController {
  constructor(private readonly service: DeviceLinkageService) {}
  @Get() list(@CurrentUser() actor: AuthUser, @Query('page') page='1', @Query('keyword') keyword='') { return this.service.list(actor, Number(page), keyword); }
  @Post() create(@CurrentUser() actor: AuthUser, @Body() dto: SaveDeviceLinkageDto) { return this.service.save(actor, dto); }
  @Patch(':id') update(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: SaveDeviceLinkageDto) { return this.service.save(actor, dto, id); }
  @Patch(':id/enabled') enable(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: EnableDeviceLinkageDto) { return this.service.enable(actor, id, dto); }
  @Delete(':id') remove(@CurrentUser() actor: AuthUser, @Param('id') id: string) { return this.service.remove(actor, id); }
  @Get(':id/runs') runs(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Query('page') page='1') { return this.service.runs(actor, id, Number(page)); }
}

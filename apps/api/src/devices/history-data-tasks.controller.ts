import { Body, Controller, Delete, Get, Param, Patch, Post, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { Permissions } from '../common/decorators/permissions.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { PermissionsGuard } from '../common/guards/permissions.guard';
import { HistoryDataTasksService, SaveHistoryTaskDto, HistoryTaskFilterDto } from './history-data-tasks.service';
@Controller('history-data-tasks')
@UseGuards(JwtAuthGuard,RolesGuard,PermissionsGuard)
@Roles('TENANT_ADMIN','TENANT_USER')
@Permissions('DEVICE_VIEW')
export class HistoryDataTasksController {
  constructor(private readonly tasks:HistoryDataTasksService){}
  @Get() list(@CurrentUser() actor:AuthUser,@Query('page') page='1',@Query('pageSize') size='20',@Query('keyword') keyword=''){return this.tasks.list(actor,Number(page),Number(size),keyword);}
  @Post() create(@CurrentUser() actor:AuthUser,@Body() dto:SaveHistoryTaskDto){return this.tasks.save(actor,dto);}
  @Get(':id') get(@CurrentUser() actor:AuthUser,@Param('id') id:string){return this.tasks.get(actor,id);}
  @Patch(':id') update(@CurrentUser() actor:AuthUser,@Param('id') id:string,@Body() dto:SaveHistoryTaskDto){return this.tasks.save(actor,dto,id);}
  @Delete(':id') remove(@CurrentUser() actor:AuthUser,@Param('id') id:string){return this.tasks.remove(actor,id);}
  @Get(':id/records') records(@CurrentUser() actor:AuthUser,@Param('id') id:string,@Query() filter:HistoryTaskFilterDto){return this.tasks.query(actor,id,Number(filter.page||1),Number(filter.pageSize||20),filter);}
  @Post(':id/export') export(@CurrentUser() actor:AuthUser,@Param('id') id:string,@Body() filter:HistoryTaskFilterDto){return this.tasks.export(actor,id,filter||{});}
}

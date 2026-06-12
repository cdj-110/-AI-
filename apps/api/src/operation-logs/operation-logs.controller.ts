import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from './operation-logs.service';

@Controller('operation-logs')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class OperationLogsController {
  constructor(private readonly operationLogsService: OperationLogsService) {}

  @Get()
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
    @Query('module') module = '',
    @Query('action') action = '',
  ) {
    return this.operationLogsService.findAll(user, Number(page), Number(pageSize), { keyword, module, action });
  }
}

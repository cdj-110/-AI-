import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { LoginLogsService } from './login-logs.service';

@Controller('login-logs')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class LoginLogsController {
  constructor(private readonly loginLogsService: LoginLogsService) {}

  @Get()
  findAll(
    @CurrentUser() user: AuthUser,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '10',
    @Query('keyword') keyword = '',
    @Query('success') success = '',
  ) {
    return this.loginLogsService.findAll(user, Number(page), Number(pageSize), keyword, success);
  }
}

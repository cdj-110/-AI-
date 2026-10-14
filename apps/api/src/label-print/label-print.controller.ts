import { Body, Controller, Delete, Get, Headers, Param, Post, Query, UseGuards } from '@nestjs/common';
import { CurrentUser, AuthUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CompletePrintJobDto } from './dto/complete-print-job.dto';
import { CreatePrintAgentDto } from './dto/create-print-agent.dto';
import { CreatePrintJobDto } from './dto/create-print-job.dto';
import { LabelPrintService } from './label-print.service';

@Controller('label-print')
export class LabelPrintController {
  constructor(private readonly service: LabelPrintService) {}

  @Get('agent/claim')
  claim(@Headers('x-print-agent-token') token = '') {
    return this.service.claim(token);
  }

  @Post('agent/jobs/:id/complete')
  complete(@Headers('x-print-agent-token') token: string, @Param('id') id: string, @Body() dto: CompletePrintJobDto) {
    return this.service.complete(token, id, dto);
  }

  @Get('agents')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  agents() { return this.service.listAgents(); }

  @Post('agents')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  createAgent(@Body() dto: CreatePrintAgentDto) { return this.service.createAgent(dto); }

  @Delete('agents/:id')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  removeAgent(@Param('id') id: string) { return this.service.removeAgent(id); }

  @Get('jobs')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  jobs(@Query('page') page = '1', @Query('pageSize') pageSize = '50') { return this.service.listJobs(Number(page), Number(pageSize)); }

  @Post('jobs')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  createJob(@CurrentUser() user: AuthUser, @Body() dto: CreatePrintJobDto) { return this.service.createJob(dto, user.username); }

  @Post('jobs/:id/retry')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  retryJob(@Param('id') id: string) { return this.service.retryJob(id); }

  @Delete('jobs/:id')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  cancelJob(@Param('id') id: string) { return this.service.cancelJob(id); }
}

import { Body, Controller, Delete, Get, Param, Patch, Post, Query, UseGuards } from '@nestjs/common';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { ChangelogService } from './changelog.service';
import { CreateChangelogEntryDto, UpdateChangelogEntryDto } from './dto/changelog.dto';

@Controller('changelog')
export class ChangelogController {
  constructor(private readonly changelog: ChangelogService) {}

  @Get()
  list(
    @Query('surface') surface = '',
    @Query('category') category = '',
    @Query('keyword') keyword = '',
  ) { return this.changelog.listPublic({ surface, category, keyword }); }

  @Get('manage')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  manage(
    @Query('surface') surface = '',
    @Query('category') category = '',
    @Query('status') status = '',
    @Query('keyword') keyword = '',
  ) { return this.changelog.listAdmin({ surface, category, status, keyword }); }

  @Post()
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  create(@CurrentUser() actor: AuthUser, @Body() dto: CreateChangelogEntryDto) { return this.changelog.create(actor, dto); }

  @Patch(':id')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  update(@Param('id') id: string, @Body() dto: UpdateChangelogEntryDto) { return this.changelog.update(id, dto); }

  @Delete(':id')
  @UseGuards(JwtAuthGuard, RolesGuard)
  @Roles('SUPER_ADMIN')
  remove(@Param('id') id: string) { return this.changelog.remove(id); }
}

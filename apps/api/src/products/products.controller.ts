import { Body, Controller, Delete, Get, Param, Patch, Post, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { Roles } from '../common/decorators/roles.decorator';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { CreateProductDto } from './dto/create-product.dto';
import { UpdateProductDto } from './dto/update-product.dto';
import { ProductsService } from './products.service';

@Controller('products')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN')
export class ProductsController {
  constructor(private readonly products: ProductsService, private readonly logs: OperationLogsService) {}

  @Get()
  findAll() { return this.products.findAll(); }

  @Post()
  async create(@Body() dto: CreateProductDto, @CurrentUser() actor: AuthUser, @Req() request: Request) {
    const product = await this.products.create(dto);
    await this.audit(actor, request, '新增产品型号', product.code, product);
    return product;
  }

  @Patch(':code')
  async update(@Param('code') code: string, @Body() dto: UpdateProductDto, @CurrentUser() actor: AuthUser, @Req() request: Request) {
    const product = await this.products.update(code, dto);
    await this.audit(actor, request, '修改产品型号', code, dto);
    return product;
  }

  @Delete(':code')
  async remove(@Param('code') code: string, @CurrentUser() actor: AuthUser, @Req() request: Request) {
    const product = await this.products.remove(code);
    await this.audit(actor, request, '删除产品型号', code, product);
    return product;
  }

  private audit(actor: AuthUser, request: Request, action: string, code: string, detail: object) {
    return this.logs.record(actor, {
      module: '产品库', action, targetType: 'ProductModel', targetId: code, targetName: code,
      ip: request.ip || request.socket.remoteAddress, userAgent: request.headers['user-agent'], detail,
    });
  }
}

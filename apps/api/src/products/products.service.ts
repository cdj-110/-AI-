import { BadRequestException, ConflictException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { normalizeGatewayModel } from '../factory-gateways/production-batch';
import { CreateProductDto } from './dto/create-product.dto';
import { UpdateProductDto } from './dto/update-product.dto';

@Injectable()
export class ProductsService {
  constructor(private readonly prisma: PrismaService) {}

  findAll() {
    return this.prisma.productModel.findMany({
      include: { _count: { select: { gateways: true } } },
      orderBy: { code: 'asc' },
    });
  }

  async create(dto: CreateProductDto) {
    if (!dto.code.trim()) throw new BadRequestException('请填写产品型号');
    let code: string;
    try { code = normalizeGatewayModel(dto.code); }
    catch (error) { throw new BadRequestException((error as Error).message); }
    if (!dto.name.trim()) throw new BadRequestException('请填写产品名称');
    try {
      return await this.prisma.productModel.create({ data: {
        ...dto, code, name: dto.name.trim(), enabled: dto.enabled ?? true,
        description: dto.description?.trim() || null,
      } });
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2002') {
        throw new ConflictException('该产品型号已存在');
      }
      throw error;
    }
  }

  async update(code: string, dto: UpdateProductDto) {
    if (dto.name !== undefined && (dto.name === null || !dto.name.trim())) throw new BadRequestException('请填写产品名称');
    if (dto.enabled === null) throw new BadRequestException('启用状态不能为空');
    try {
      return await this.prisma.productModel.update({ where: { code }, data: {
        ...dto,
        ...(dto.name !== undefined ? { name: dto.name.trim() } : {}),
        ...(dto.description !== undefined ? { description: dto.description?.trim() || null } : {}),
      } });
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2025') throw new NotFoundException('产品型号不存在');
      throw error;
    }
  }

  async remove(code: string) {
    // The foreign key also prevents concurrent registration/deletion races.
    try { return await this.prisma.productModel.delete({ where: { code } }); }
    catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError) {
        if (error.code === 'P2025') throw new NotFoundException('产品型号不存在');
        if (error.code === 'P2003') throw new ConflictException('该型号已有出厂网关引用，不能删除；可停用以禁止新增登记');
      }
      throw error;
    }
  }
}

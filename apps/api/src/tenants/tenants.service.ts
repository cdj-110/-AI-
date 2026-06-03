import { Injectable, NotFoundException } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import { CreateTenantDto } from './dto/create-tenant.dto';
import { UpdateTenantDto } from './dto/update-tenant.dto';

@Injectable()
export class TenantsService {
  constructor(private readonly prisma: PrismaService) {}

  findAll() {
    return this.prisma.tenant.findMany({ include: { _count: { select: { users: true } } }, orderBy: { createdAt: 'desc' } });
  }

  create(dto: CreateTenantDto) {
    return this.prisma.tenant.create({ data: dto });
  }

  async findOne(id: string) {
    const tenant = await this.prisma.tenant.findUnique({ where: { id }, include: { _count: { select: { users: true } } } });
    if (!tenant) throw new NotFoundException('租户不存在');
    return tenant;
  }

  async update(id: string, dto: UpdateTenantDto) {
    await this.findOne(id);
    return this.prisma.tenant.update({ where: { id }, data: dto });
  }

  async remove(id: string) {
    await this.findOne(id);
    await this.prisma.tenant.delete({ where: { id } });
    return { message: '删除成功' };
  }
}

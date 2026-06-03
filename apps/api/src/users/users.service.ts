import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma, User } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';

@Injectable()
export class UsersService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(actor: AuthUser, page: number, pageSize: number, keyword: string) {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.UserWhereInput = {
      ...(actor.role === 'TENANT_ADMIN' ? { tenantId: actor.tenantId } : {}),
      ...(keyword
        ? { OR: [{ username: { contains: keyword, mode: 'insensitive' } }, { nickname: { contains: keyword, mode: 'insensitive' } }] }
        : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.user.findMany({ where, skip: (safePage - 1) * safePageSize, take: safePageSize, orderBy: { createdAt: 'desc' } }),
      this.prisma.user.count({ where }),
    ]);
    return { items: items.map(this.toPublicUser), total, page: safePage, pageSize: safePageSize };
  }

  async create(actor: AuthUser, dto: CreateUserDto) {
    this.assertWritableRole(actor, dto.role);
    const tenantId = actor.role === 'TENANT_ADMIN' ? actor.tenantId : dto.tenantId;
    if (dto.role !== 'SUPER_ADMIN' && !tenantId) throw new BadRequestException('租户用户必须指定 tenantId');
    const exists = await this.prisma.user.findUnique({ where: { username: dto.username } });
    if (exists) throw new BadRequestException('用户名已存在');
    const user = await this.prisma.user.create({
      data: { ...dto, tenantId, password: await bcrypt.hash(dto.password, 10) },
    });
    return this.toPublicUser(user);
  }

  async findOne(actor: AuthUser, id: string) {
    const user = await this.getAccessibleUser(actor, id);
    return this.toPublicUser(user);
  }

  async update(actor: AuthUser, id: string, dto: UpdateUserDto) {
    const target = await this.getAccessibleUser(actor, id);
    if (dto.role) this.assertWritableRole(actor, dto.role);
    if (actor.role === 'TENANT_ADMIN' && dto.tenantId && dto.tenantId !== actor.tenantId) {
      throw new ForbiddenException('不能移动其他租户的用户');
    }
    const { password, ...rest } = dto;
    const user = await this.prisma.user.update({
      where: { id: target.id },
      data: { ...rest, ...(password ? { password: await bcrypt.hash(password, 10) } : {}) },
    });
    return this.toPublicUser(user);
  }

  async remove(actor: AuthUser, id: string) {
    const target = await this.getAccessibleUser(actor, id);
    if (actor.sub === id) throw new BadRequestException('不能删除当前登录用户');
    await this.prisma.user.delete({ where: { id: target.id } });
    return { message: '删除成功' };
  }

  private async getAccessibleUser(actor: AuthUser, id: string) {
    const user = await this.prisma.user.findUnique({ where: { id } });
    if (!user) throw new NotFoundException('用户不存在');
    if (actor.role === 'TENANT_ADMIN' && user.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该用户');
    return user;
  }

  private assertWritableRole(actor: AuthUser, role: string) {
    if (actor.role === 'TENANT_ADMIN' && role !== 'TENANT_USER') {
      throw new ForbiddenException('租户管理员只能管理普通租户用户');
    }
  }

  private toPublicUser(user: User) {
    const { password: _password, ...publicUser } = user;
    return publicUser;
  }
}

import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateChangelogEntryDto, UpdateChangelogEntryDto } from './dto/changelog.dto';

const validSurfaces = ['ADMIN', 'TENANT', 'GATEWAY'];
const validCategories = ['FEATURE', 'IMPROVEMENT', 'FIX', 'SECURITY'];
const validStatuses = ['DRAFT', 'PUBLISHED'];

@Injectable()
export class ChangelogService {
  constructor(private readonly prisma: PrismaService) {}

  listPublic(filters: { surface?: string; category?: string; keyword?: string }) {
    return this.list({ ...filters, status: 'PUBLISHED' }, false);
  }

  listAdmin(filters: { surface?: string; category?: string; status?: string; keyword?: string }) {
    return this.list(filters, true);
  }

  private async list(filters: { surface?: string; category?: string; status?: string; keyword?: string }, allowAllStatuses: boolean) {
    const surface = validSurfaces.includes(filters.surface || '') ? filters.surface : undefined;
    const category = validCategories.includes(filters.category || '') ? filters.category : undefined;
    const requestedStatus = validStatuses.includes(filters.status || '') ? filters.status : undefined;
    const status = allowAllStatuses ? requestedStatus : 'PUBLISHED';
    const keyword = filters.keyword?.trim();
    const where: Prisma.ChangelogEntryWhereInput = {
      ...(status ? { status } : {}),
      ...(surface ? { surfaces: { has: surface } } : {}),
      ...(category ? { category } : {}),
      ...(keyword ? { OR: [
        { title: { contains: keyword, mode: 'insensitive' } },
        { summary: { contains: keyword, mode: 'insensitive' } },
        { version: { contains: keyword, mode: 'insensitive' } },
      ] } : {}),
    };
    const [items, total] = await this.prisma.$transaction([
      this.prisma.changelogEntry.findMany({ where, orderBy: [{ releaseDate: 'desc' }, { createdAt: 'desc' }], take: 300 }),
      this.prisma.changelogEntry.count({ where }),
    ]);
    return { items, total };
  }

  async create(actor: AuthUser, dto: CreateChangelogEntryDto) {
    const normalizedItems = dto.items.map(item => ({ text: item.text.trim() })).filter(item => item.text);
    if (!normalizedItems.length) throw new BadRequestException('至少填写一条更新内容');
    const status = dto.status || 'DRAFT';
    return this.prisma.changelogEntry.create({ data: {
      version: dto.version?.trim() || null,
      title: dto.title.trim(),
      summary: dto.summary?.trim() || null,
      releaseDate: new Date(dto.releaseDate),
      surfaces: [...new Set(dto.surfaces)],
      category: dto.category,
      items: normalizedItems,
      status,
      createdById: actor.sub,
      createdByName: actor.username,
      publishedAt: status === 'PUBLISHED' ? new Date() : null,
    } });
  }

  async update(id: string, dto: UpdateChangelogEntryDto) {
    const existing = await this.prisma.changelogEntry.findUnique({ where: { id } });
    if (!existing) throw new NotFoundException('更新日志不存在');
    const data = this.normalize(dto, true);
    return this.prisma.changelogEntry.update({ where: { id }, data: {
      ...data,
      ...(dto.status ? { publishedAt: dto.status === 'PUBLISHED' ? existing.publishedAt || new Date() : null } : {}),
    } });
  }

  async remove(id: string) {
    const existing = await this.prisma.changelogEntry.findUnique({ where: { id }, select: { id: true } });
    if (!existing) throw new NotFoundException('更新日志不存在');
    await this.prisma.changelogEntry.delete({ where: { id } });
    return { success: true };
  }

  private normalize(dto: CreateChangelogEntryDto | UpdateChangelogEntryDto, partial = false) {
    const items = dto.items?.map(item => ({ text: item.text.trim() })).filter(item => item.text);
    if (dto.items && !items?.length) throw new BadRequestException('至少填写一条更新内容');
    return {
      ...(dto.version !== undefined ? { version: dto.version.trim() || null } : {}),
      ...(dto.title !== undefined ? { title: dto.title.trim() } : {}),
      ...(dto.summary !== undefined ? { summary: dto.summary.trim() || null } : {}),
      ...(dto.releaseDate !== undefined ? { releaseDate: new Date(dto.releaseDate) } : {}),
      ...(dto.surfaces !== undefined ? { surfaces: [...new Set(dto.surfaces)] } : {}),
      ...(dto.category !== undefined ? { category: dto.category } : {}),
      ...(items !== undefined ? { items: items as unknown as Prisma.InputJsonValue } : {}),
      ...(dto.status !== undefined ? { status: dto.status } : (!partial ? { status: 'DRAFT' } : {})),
    };
  }
}

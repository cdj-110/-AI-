import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateTemplateFromDeviceDto } from './dto/create-template-from-device.dto';
import { CreateModelTemplateDto, ModelTemplateMetricDto } from './dto/create-model-template.dto';
import { UpdateModelTemplateDto } from './dto/update-model-template.dto';

@Injectable()
export class ModelTemplatesService {
  constructor(private readonly prisma: PrismaService) {}

  // 物模型模板是“可复用字段集合”，创建设备时会复制到设备物模型中。
  async findAll(actor: AuthUser, keyword = '') {
    const where: Prisma.DeviceModelTemplateWhereInput = {
      ...(actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {}),
      ...(keyword ? { name: { contains: keyword, mode: 'insensitive' } } : {}),
    };
    return this.prisma.deviceModelTemplate.findMany({
      where,
      include: {
        tenant: { select: { id: true, name: true } },
        _count: { select: { metrics: true } },
      },
      orderBy: { createdAt: 'desc' },
    });
  }

  async options(actor: AuthUser) {
    return this.prisma.deviceModelTemplate.findMany({
      where: actor.role !== 'SUPER_ADMIN' ? { tenantId: actor.tenantId ?? undefined } : {},
      select: { id: true, name: true, deviceType: true, _count: { select: { metrics: true } } },
      orderBy: { createdAt: 'desc' },
      take: 200,
    });
  }

  async findOne(actor: AuthUser, id: string) {
    const template = await this.getAccessibleTemplate(actor, id);
    return template;
  }

  async create(actor: AuthUser, dto: CreateModelTemplateDto) {
    const tenantId = actor.role === 'SUPER_ADMIN' ? await this.getDefaultTenantId() : actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('物模型模板必须归属于租户');
    return this.prisma.deviceModelTemplate.create({
      data: {
        tenantId,
        name: dto.name,
        description: dto.description || undefined,
        deviceType: dto.deviceType ?? 'DIRECT',
        metrics: dto.metrics?.length ? { create: this.normalizeMetrics(dto.metrics) } : undefined,
      },
      include: { metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] } },
    });
  }

  async duplicate(actor: AuthUser, id: string) {
    const source = await this.getAccessibleTemplate(actor, id);
    return this.prisma.deviceModelTemplate.create({
      data: {
        tenantId: source.tenantId,
        name: `${source.name} 副本`,
        description: source.description,
        deviceType: source.deviceType,
        metrics: source.metrics.length ? { create: this.normalizeMetrics(source.metrics) } : undefined,
      },
      include: { metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] } },
    });
  }

  async createFromDevice(actor: AuthUser, dto: CreateTemplateFromDeviceDto) {
    // 支持把真实设备自动发现出来的字段沉淀成模板，方便后续批量复用。
    const device = await this.prisma.device.findUnique({
      where: { id: dto.deviceId },
      include: { metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] } },
    });
    if (!device) throw new NotFoundException('设备不存在');
    if (actor.role !== 'SUPER_ADMIN' && device.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该设备');
    if (!device.metrics.length) throw new BadRequestException('该设备暂无可生成模板的物模型字段');
    return this.prisma.deviceModelTemplate.create({
      data: {
        tenantId: device.tenantId,
        name: dto.name?.trim() || `${device.name} 物模型`,
        description: dto.description || `从设备 ${device.name} (${device.deviceKey}) 生成`,
        deviceType: device.deviceType,
        metrics: { create: this.normalizeMetrics(device.metrics) },
      },
      include: { metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] } },
    });
  }

  async update(actor: AuthUser, id: string, dto: UpdateModelTemplateDto) {
    const template = await this.getAccessibleTemplate(actor, id);
    return this.prisma.deviceModelTemplate.update({
      where: { id: template.id },
      data: {
        name: dto.name,
        description: dto.description,
        deviceType: dto.deviceType,
      },
      include: { metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] } },
    });
  }

  async saveMetrics(actor: AuthUser, id: string, metrics: ModelTemplateMetricDto[]) {
    const template = await this.getAccessibleTemplate(actor, id);
    const normalized = this.normalizeMetrics(metrics);
    await this.prisma.$transaction(async (tx) => {
      // 模板字段采用整表替换，前端抽屉里删除字段后保存即可生效。
      await tx.deviceModelMetric.deleteMany({ where: { templateId: template.id } });
      if (normalized.length) {
        await tx.deviceModelMetric.createMany({
          data: normalized.map((metric) => ({ templateId: template.id, ...metric })),
        });
      }
    });
    return this.findOne(actor, template.id);
  }

  async remove(actor: AuthUser, id: string) {
    const template = await this.getAccessibleTemplate(actor, id);
    await this.prisma.deviceModelTemplate.delete({ where: { id: template.id } });
    return { message: '删除成功' };
  }

  private normalizeMetrics(metrics: Array<{
    identifier: string;
    name: string;
    dataType: string;
    unit?: string | null;
    decimals?: number;
    accessMode?: string;
    enabled?: boolean;
    sortOrder?: number;
  }>) {
    // 统一字段默认值和排序，避免不同入口创建出来的物模型结构不一致。
    const seen = new Set<string>();
    return metrics.map((metric, index) => {
      const identifier = metric.identifier.trim();
      if (!identifier) throw new BadRequestException('字段标识符不能为空');
      if (seen.has(identifier)) throw new BadRequestException(`字段标识符重复：${identifier}`);
      seen.add(identifier);
      return {
        identifier,
        name: metric.name.trim() || identifier,
        dataType: metric.dataType,
        unit: metric.unit || null,
        decimals: metric.decimals ?? 2,
        accessMode: metric.accessMode ?? 'READ_ONLY',
        enabled: metric.enabled ?? true,
        sortOrder: metric.sortOrder ?? (index + 1) * 10,
      };
    });
  }

  private async getAccessibleTemplate(actor: AuthUser, id: string) {
    const template = await this.prisma.deviceModelTemplate.findUnique({
      where: { id },
      include: {
        tenant: { select: { id: true, name: true } },
        metrics: { orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }] },
      },
    });
    if (!template) throw new NotFoundException('物模型模板不存在');
    if (actor.role !== 'SUPER_ADMIN' && template.tenantId !== actor.tenantId) throw new ForbiddenException('无权访问该物模型模板');
    return template;
  }

  private async getDefaultTenantId() {
    const tenant = await this.prisma.tenant.findFirst({ where: { name: '系统默认租户' } });
    if (tenant) return tenant.id;
    return (await this.prisma.tenant.create({ data: { name: '系统默认租户' } })).id;
  }
}

import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { CreateScadaProjectDto } from './dto/create-scada-project.dto';
import { UpdateScadaProjectDto } from './dto/update-scada-project.dto';
import { deviceAccessWhere } from '../common/user-permissions';
import { SubscriptionLimitsService } from '../subscriptions/subscription-limits.service';

const EMPTY_SCADA_CONFIG = {
  version: 1,
  canvas: { width: 1280, height: 720, background: '#081225' },
  widgets: [],
};

@Injectable()
export class ScadaProjectsService {
  constructor(private readonly prisma: PrismaService, private readonly subscriptionLimits: SubscriptionLimitsService) {}

  async list(actor: AuthUser, keyword = '') {
    const tenantId = await this.resolveTenantId(actor);
    await this.subscriptionLimits.assertCanAddScadaProjects(tenantId);
    return this.prisma.scadaProject.findMany({
      where: {
        tenantId,
        ...(keyword.trim() ? { name: { contains: keyword.trim(), mode: 'insensitive' } } : {}),
      },
      select: {
        id: true,
        name: true,
        description: true,
        status: true,
        thumbnail: true,
        createdBy: true,
        updatedBy: true,
        createdAt: true,
        updatedAt: true,
      },
      orderBy: { updatedAt: 'desc' },
    });
  }

  async findOne(actor: AuthUser, id: string) {
    return this.getAccessible(actor, id);
  }

  async create(actor: AuthUser, dto: CreateScadaProjectDto) {
    const tenantId = await this.resolveTenantId(actor);
    return this.prisma.scadaProject.create({
      data: {
        tenantId,
        name: this.cleanName(dto.name),
        description: dto.description?.trim() || null,
        config: EMPTY_SCADA_CONFIG,
        createdBy: actor.sub,
        updatedBy: actor.sub,
      },
    });
  }

  async update(actor: AuthUser, id: string, dto: UpdateScadaProjectDto) {
    const project = await this.getAccessible(actor, id);
    if (dto.config && JSON.stringify(dto.config).length > 2_000_000) {
      throw new BadRequestException('组态配置不能超过 2 MB');
    }
    return this.prisma.scadaProject.update({
      where: { id: project.id },
      data: {
        ...(dto.name !== undefined ? { name: this.cleanName(dto.name) } : {}),
        ...(dto.description !== undefined ? { description: dto.description.trim() || null } : {}),
        ...(dto.status !== undefined ? { status: dto.status } : {}),
        ...(dto.thumbnail !== undefined ? { thumbnail: dto.thumbnail || null } : {}),
        ...(dto.config !== undefined ? { config: dto.config as Prisma.InputJsonValue } : {}),
        updatedBy: actor.sub,
      },
    });
  }

  async duplicate(actor: AuthUser, id: string) {
    const source = await this.getAccessible(actor, id);
    await this.subscriptionLimits.assertCanAddScadaProjects(source.tenantId);
    return this.prisma.scadaProject.create({
      data: {
        tenantId: source.tenantId,
        name: `${source.name} 副本`.slice(0, 80),
        description: source.description,
        status: 'DRAFT',
        thumbnail: source.thumbnail,
        config: source.config as Prisma.InputJsonValue,
        createdBy: actor.sub,
        updatedBy: actor.sub,
      },
    });
  }

  async remove(actor: AuthUser, id: string) {
    const project = await this.getAccessible(actor, id);
    await this.prisma.scadaProject.delete({ where: { id: project.id } });
    return { message: '组态项目已删除' };
  }

  async goViewConfig(actor: AuthUser, id: string) {
    const project = await this.getAccessible(actor, id);
    const config = project.config && typeof project.config === 'object' && !Array.isArray(project.config)
      ? project.config as Record<string, unknown>
      : {};
    const mainData = config.main_data && typeof config.main_data === 'object'
      ? config.main_data as Record<string, unknown>
      : { item: [] };
    return {
      main_data: {
        ...mainData,
        item: Array.isArray(mainData.item) ? mainData.item : [],
      },
      session_id: project.id,
    };
  }

  async saveGoViewConfig(actor: AuthUser, id: string, mainData: unknown, thumbnail?: string) {
    const project = await this.getAccessible(actor, id);
    const normalized = mainData && typeof mainData === 'object' && !Array.isArray(mainData)
      ? mainData as Record<string, unknown>
      : { item: [] };
    if (JSON.stringify(normalized).length > 2_000_000) {
      throw new BadRequestException('组态配置不能超过 2 MB');
    }
    const normalizedThumbnail = typeof thumbnail === 'string' ? thumbnail.trim() : '';
    if (normalizedThumbnail.length > 500_000) {
      throw new BadRequestException('组态预览图不能超过 500 KB');
    }
    await this.prisma.scadaProject.update({
      where: { id: project.id },
      data: {
        config: { provider: 'goview', main_data: normalized } as Prisma.InputJsonValue,
        ...(normalizedThumbnail ? { thumbnail: normalizedThumbnail } : {}),
        updatedBy: actor.sub,
      },
    });
  }

  async goViewDevices(actor: AuthUser) {
    const devices = await this.prisma.device.findMany({
      where: deviceAccessWhere(actor),
      select: { id: true, name: true, deviceKey: true, deviceType: true },
      orderBy: [{ name: 'asc' }, { createdAt: 'asc' }],
    });
    return devices.map(device => ({
      dev_id: device.id,
      dev_name: device.name,
      dev_key: device.deviceKey,
      dev_type: device.deviceType,
      model_id: device.id,
    }));
  }

  async goViewMetrics(actor: AuthUser, deviceId: string) {
    const device = await this.prisma.device.findFirst({
      where: { AND: [deviceAccessWhere(actor), { id: deviceId }] },
      select: { id: true },
    });
    if (!device) throw new NotFoundException('设备不存在或无权访问');
    return this.prisma.deviceMetric.findMany({
      where: { deviceId: device.id, enabled: true, ignored: false },
      select: { identifier: true, name: true, dataType: true, unit: true, decimals: true },
      orderBy: [{ sortOrder: 'asc' }, { createdAt: 'asc' }],
    });
  }

  private cleanName(name: string) {
    const value = name.trim();
    if (!value) throw new BadRequestException('请输入项目名称');
    return value;
  }

  private async getAccessible(actor: AuthUser, id: string) {
    const tenantId = await this.resolveTenantId(actor);
    const project = await this.prisma.scadaProject.findFirst({ where: { id, tenantId } });
    if (!project) throw new NotFoundException('组态项目不存在或无权访问');
    return project;
  }

  private async resolveTenantId(actor: AuthUser) {
    if (actor.role !== 'SUPER_ADMIN') {
      if (!actor.tenantId) throw new ForbiddenException('当前账号未关联租户');
      return actor.tenantId;
    }
    const tenant = await this.prisma.tenant.findFirst({ orderBy: { createdAt: 'asc' }, select: { id: true } });
    if (!tenant) throw new BadRequestException('请先创建租户');
    return tenant.id;
  }
}

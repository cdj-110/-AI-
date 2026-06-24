import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { randomBytes } from 'crypto';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { BindFactoryGatewayDto } from './dto/bind-factory-gateway.dto';
import { RegisterFactoryGatewayDto } from './dto/register-factory-gateway.dto';

@Injectable()
export class FactoryGatewaysService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(page: number, pageSize: number, keyword = '') {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.FactoryGatewayWhereInput = keyword
      ? { OR: [{ hardwareId: { contains: keyword, mode: 'insensitive' } }, { sn: { contains: keyword, mode: 'insensitive' } }] }
      : {};
    const [items, total] = await this.prisma.$transaction([
      this.prisma.factoryGateway.findMany({
        where,
        include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
        orderBy: { createdAt: 'desc' },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
      }),
      this.prisma.factoryGateway.count({ where }),
    ]);
    return { items: items.map((item) => this.toPublicFactoryGateway(item)), total, page: safePage, pageSize: safePageSize };
  }

  async register(dto: RegisterFactoryGatewayDto) {
    const hardwareId = this.normalizeHardwareId(dto.hardwareId);
    const exists = await this.prisma.factoryGateway.findUnique({ where: { hardwareId } });
    if (exists) throw new BadRequestException('该 HardwareId 已经登记');

    const sn = await this.generateUniqueSN();
    const deviceSecret = this.randomSecret(24);
    const bindCode = this.randomBindCode();
    const created = await this.prisma.factoryGateway.create({
      data: {
        hardwareId,
        sn,
        batchNo: dto.batchNo?.trim() || undefined,
        producedAt: new Date(),
        deviceSecretHash: await bcrypt.hash(deviceSecret, 10),
        bindCodeHash: await bcrypt.hash(bindCode, 10),
      },
    });
    return {
      ...this.toPublicFactoryGateway(created),
      deviceSecret,
      bindCode,
      activationFile: {
        enabled: true,
        hardwareId,
        sn,
        deviceSecret,
        broker: this.getPublicMqttBroker(),
      },
    };
  }

  async bind(actor: AuthUser, dto: BindFactoryGatewayDto) {
    const tenantId = actor.role === 'SUPER_ADMIN' ? await this.getDefaultTenantId() : actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('当前账号没有可绑定的租户');

    const sn = dto.sn.trim();
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { sn } });
    if (!factoryGateway) throw new NotFoundException('SN 不存在');
    if (factoryGateway.status === 'BOUND' || factoryGateway.deviceId) throw new BadRequestException('该网关已绑定');
    if (!(await bcrypt.compare(dto.bindCode, factoryGateway.bindCodeHash))) throw new BadRequestException('绑定码不正确');

    const name = dto.name?.trim() || `网关 ${sn}`;
    const onlineThresholdMs = Number(process.env.DEVICE_OFFLINE_TIMEOUT_SECONDS ?? 90) * 1000;
    const isRecentlyOnline = Boolean(
      factoryGateway.lastSeenAt && Date.now() - factoryGateway.lastSeenAt.getTime() <= onlineThresholdMs,
    );
    return this.prisma.$transaction(async (tx) => {
      const device = await tx.device.create({
        data: {
          tenantId,
          deviceKey: sn,
          name,
          location: dto.location?.trim() || undefined,
          deviceType: 'GATEWAY',
          protocol: 'MQTT',
          status: isRecentlyOnline ? 'ONLINE' : 'OFFLINE',
          lastSeenAt: factoryGateway.lastSeenAt,
          mqttClientId: `gw_${sn}`,
          mqttUsername: `gateway:${sn}`,
          mqttPasswordHash: factoryGateway.deviceSecretHash,
          mqttPasswordUpdatedAt: new Date(),
        },
      });
      const updated = await tx.factoryGateway.update({
        where: { id: factoryGateway.id },
        data: { status: 'BOUND', boundAt: new Date(), deviceId: device.id },
        include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
      });
      return { factoryGateway: this.toPublicFactoryGateway(updated), device };
    });
  }

  async resetBindCode(id: string) {
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!factoryGateway) throw new NotFoundException('出厂网关不存在');
    if (factoryGateway.status === 'BOUND' || factoryGateway.deviceId) throw new BadRequestException('该网关已绑定，不能重置绑定码');

    const bindCode = this.randomBindCode();
    const updated = await this.prisma.factoryGateway.update({
      where: { id },
      data: { bindCodeHash: await bcrypt.hash(bindCode, 10) },
      include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
    });
    return { ...this.toPublicFactoryGateway(updated), bindCode };
  }

  async resetActivation(id: string) {
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!factoryGateway) throw new NotFoundException('出厂网关不存在');
    if (factoryGateway.status === 'BOUND' || factoryGateway.deviceId) {
      throw new BadRequestException('该网关已绑定，不能重新生成激活文件');
    }

    const deviceSecret = this.randomSecret(24);
    const updated = await this.prisma.factoryGateway.update({
      where: { id },
      data: { deviceSecretHash: await bcrypt.hash(deviceSecret, 10) },
      include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
    });
    return {
      ...this.toPublicFactoryGateway(updated),
      deviceSecret,
      activationFile: {
        enabled: true,
        hardwareId: updated.hardwareId,
        sn: updated.sn,
        deviceSecret,
        broker: this.getPublicMqttBroker(),
      },
    };
  }

  async markFactoryOnline(hardwareId: string) {
    const normalized = this.normalizeHardwareId(hardwareId);
    const now = new Date();
    const gateway = await this.prisma.factoryGateway.update({
      where: { hardwareId: normalized },
      data: { firstSeenAt: { set: now }, lastSeenAt: now },
      include: { device: true },
    });
    if (gateway.deviceId) {
      await this.prisma.device.update({
        where: { id: gateway.deviceId },
        data: { status: 'ONLINE', lastSeenAt: now },
      });
    }
    return gateway;
  }

  private normalizeHardwareId(value: string) {
    const hardwareId = value.trim().toUpperCase();
    if (!hardwareId) throw new BadRequestException('HardwareId 不能为空');
    if (!/^[0-9A-F]{32}$/.test(hardwareId)) {
      throw new BadRequestException('HardwareId 必须是网关页面读取到的完整 32 位硬件指纹');
    }
    return hardwareId;
  }

  private async generateUniqueSN() {
    for (;;) {
      const sn = `WK${new Date().getFullYear()}${randomBytes(4).toString('hex').toUpperCase()}`;
      const exists = await this.prisma.factoryGateway.findUnique({ where: { sn } });
      if (!exists) return sn;
    }
  }

  private randomSecret(length: number) {
    return randomBytes(length).toString('base64url');
  }

  private randomBindCode() {
    return String(Math.floor(100000 + Math.random() * 900000));
  }

  private getPublicMqttBroker() {
    const host = process.env.MQTT_PUBLIC_HOST ?? '127.0.0.1';
    const port = Number(process.env.MQTT_PUBLIC_PORT ?? 1883);
    return `tcp://${host}:${port}`;
  }

  private async getDefaultTenantId() {
    const tenant = await this.prisma.tenant.findFirst({ where: { name: '系统默认租户' } });
    if (tenant) return tenant.id;
    return (await this.prisma.tenant.create({ data: { name: '系统默认租户' } })).id;
  }

  private toPublicFactoryGateway<T extends { deviceSecretHash?: string; bindCodeHash?: string }>(gateway: T) {
    const { deviceSecretHash: _deviceSecretHash, bindCodeHash: _bindCodeHash, ...safe } = gateway;
    return safe;
  }
}

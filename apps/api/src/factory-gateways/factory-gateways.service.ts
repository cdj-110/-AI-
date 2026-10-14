import { BadRequestException, ConflictException, Injectable, InternalServerErrorException, NotFoundException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Prisma } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { createCipheriv, createDecipheriv, createHash, createPrivateKey, randomBytes, sign } from 'crypto';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { BindFactoryGatewayDto } from './dto/bind-factory-gateway.dto';
import { RegisterFactoryGatewayDto } from './dto/register-factory-gateway.dto';
import { BackfillGatewayIdentityDto } from './dto/backfill-gateway-identity.dto';
import { highestBatchSequence, normalizeGatewayModel, productionBatchPrefix } from './production-batch';
import { authorizationStatus, LicenseStatusInput } from './license-status';
import { SubscriptionLimitsService } from '../subscriptions/subscription-limits.service';

type LicenseProductProfile = {
  code: string;
  ethernetPortCount: number | null;
  rs232PortCount: number | null;
  rs485PortCount: number | null;
  canPortCount: number | null;
  supports4G: boolean | null;
  supportsWifi: boolean | null;
};

type LicenseHardwareProfile = {
  ethernetPortCount: number;
  rs232PortCount: number;
  rs485PortCount: number;
  canPortCount: number;
  supports4G: boolean;
  supportsWifi: boolean;
};

@Injectable()
export class FactoryGatewaysService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly config: ConfigService,
    private readonly subscriptionLimits: SubscriptionLimitsService,
  ) {}

  async findAll(page: number, pageSize: number, keyword = '') {
    const safePage = Math.max(1, Number.isFinite(page) ? page : 1);
    const safePageSize = Math.min(100, Math.max(1, Number.isFinite(pageSize) ? pageSize : 10));
    const where: Prisma.FactoryGatewayWhereInput = {
      status: { not: 'REMOVED' },
      ...(keyword
        ? { OR: [{ hardwareId: { contains: keyword, mode: 'insensitive' as const } }, { chipUsid: { contains: keyword, mode: 'insensitive' as const } }, { sn: { contains: keyword, mode: 'insensitive' as const } }, { batchNo: { contains: keyword, mode: 'insensitive' as const } }] }
        : {}),
    };
    const [items, total, statsTotal, boundTotal] = await this.prisma.$transaction([
      this.prisma.factoryGateway.findMany({
        where,
        include: {
          product: true,
          device: {
            select: {
              id: true,
              name: true,
              deviceKey: true,
              tenantId: true,
              status: true,
              tenant: { select: { id: true, name: true } },
            },
          },
          otaTasks: {
            where: { status: 'SUCCEEDED' },
            include: {
              package: { select: { version: true, release: true } },
            },
            orderBy: [{ completedAt: 'desc' }, { createdAt: 'desc' }],
            take: 1,
          },
        },
        orderBy: { createdAt: 'desc' },
        skip: (safePage - 1) * safePageSize,
        take: safePageSize,
      }),
      this.prisma.factoryGateway.count({ where }),
      this.prisma.factoryGateway.count({ where: { status: { not: 'REMOVED' } } }),
      this.prisma.factoryGateway.count({ where: { status: { not: 'REMOVED' }, deviceId: { not: null } } }),
    ]);
    return {
      items: items.map((item) => {
        const { otaTasks, ...gateway } = item;
        const installed = otaTasks[0];
        return {
          ...this.toPublicFactoryGateway(gateway),
          currentVersion: installed?.package.version,
          currentRelease: installed?.package.release.toString(),
          versionConfirmedAt: installed?.completedAt ?? undefined,
          boundDevice: item.device
            ? { id: item.device.id, name: item.device.name, deviceKey: item.device.deviceKey, status: item.device.status }
            : undefined,
          boundTenant: item.device?.tenant ?? undefined,
        };
      }),
      total,
      page: safePage,
      pageSize: safePageSize,
      stats: { total: statsTotal, bound: boundTotal, unbound: Math.max(0, statsTotal - boundTotal) },
    };
  }

  async register(dto: RegisterFactoryGatewayDto) {
    let model: string;
    try { model = normalizeGatewayModel(dto.model); }
    catch (error) { throw new BadRequestException((error as Error).message); }
    const hardwareId = this.normalizeHardwareId(dto.hardwareId);
	const chipUsid = this.normalizeChipUsid(dto.chipUsid);
    const exists = await this.prisma.factoryGateway.findUnique({ where: { hardwareId } });
    if (exists && exists.status !== 'REMOVED') throw new BadRequestException('该 HardwareId 已经登记');
	const chipExists = await this.prisma.factoryGateway.findUnique({ where: { chipUsid } });
	if (chipExists && chipExists.id !== exists?.id) {
	  throw new BadRequestException('该 RJGT102 芯片 USID 已经登记');
	}

    const sn = exists?.sn ?? await this.generateUniqueSN();
    const deviceSecret = this.randomSecret(24);
    const bindCode = this.randomBindCode();
    const licenseSequence = (exists?.licenseSequence ?? 0) + 1;
    const producedAt = new Date();
    const data = {
        reportedLicense: Prisma.DbNull,
        licenseActivatedAt: null,
        licenseReportedAt: null,
        activationSecretEncrypted: this.encryptCredential(deviceSecret, 'activation-secret'),
        hardwareId,
		chipUsid,
        sn,
        model,
        producedAt,
        deviceSecretHash: await bcrypt.hash(deviceSecret, 10),
        bindCodeHash: await bcrypt.hash(bindCode, 10),
        bindCodeEncrypted: this.encryptCredential(bindCode, 'bind-code'),
        nfcTokenHash: null,
        nfcTokenEncrypted: null,
        nfcIssuedAt: null,
        deviceId: null,
        boundAt: null,
        status: 'UNBOUND',
      };
    const result = await this.prisma.$transaction(async tx => {
      const product = await tx.productModel.findUnique({ where: { code: model } });
      if (!product) throw new BadRequestException('产品型号不存在，请先在产品库登记');
      if (!product.enabled) throw new BadRequestException('该产品型号已停用，不能新增出厂登记');
      const activationFile = this.createActivationFile({ hardwareId, chipUsid, sn, deviceSecret, licenseSequence, product });
      let batchNo = dto.batchNo?.trim();
      if (!batchNo) {
        const prefix = productionBatchPrefix(model, producedAt);
        const batches = await tx.factoryGateway.findMany({
          where: { batchNo: { startsWith: prefix } }, select: { batchNo: true },
        });
        // A single atomic PostgreSQL upsert serializes concurrent allocations.
        // Existing matching batch numbers seed the initial counter after upgrade.
        const [counter] = await tx.$queryRaw<{ lastSequence: number }[]>(Prisma.sql`
          INSERT INTO "FactoryProductionCounter" ("prefix", "lastSequence")
          VALUES (${prefix}, ${highestBatchSequence(prefix, batches) + 1})
          ON CONFLICT ("prefix") DO UPDATE SET "lastSequence" =
            GREATEST("FactoryProductionCounter"."lastSequence", EXCLUDED."lastSequence" - 1) + 1
          RETURNING "lastSequence"
        `);
        batchNo = `${prefix}${String(counter.lastSequence).padStart(3, '0')}`;
      }
      const registrationData = {
        ...data,
        batchNo,
        licenseSequence,
        issuedLicense: this.publicLicenseMetadata(activationFile),
      };
      const gateway = exists
        ? tx.factoryGateway.update({ where: { id: exists.id, licenseSequence: exists.licenseSequence, status: 'REMOVED' }, data: registrationData })
        : tx.factoryGateway.create({ data: registrationData });
      return { gateway: await gateway, activationFile };
    });
    return {
      ...this.toPublicFactoryGateway(result.gateway),
      deviceSecret,
      bindCode,
      activationFile: result.activationFile,
    };
  }

  async bind(actor: AuthUser, dto: BindFactoryGatewayDto) {
    const tenantId = actor.tenantId ?? undefined;
    if (!tenantId) throw new BadRequestException('当前账号未关联租户，不能绑定网关');

    const factoryGateway = await this.resolveBindableGateway(dto);
    await this.subscriptionLimits.assertCanAddDevices(tenantId);
    const sn = factoryGateway.sn;

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
        // NFC 标签是网关的可复用归属凭证：已绑定期间由状态和 deviceId 阻止重复绑定，
        // 解绑后保留的凭证可再次用于绑定，避免将实体标签变成一次性耗材。
        data: { status: 'BOUND', boundAt: new Date(), deviceId: device.id },
        include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
      });
      return { factoryGateway: this.toPublicFactoryGateway(updated), device };
    });
  }

  async verifyBind(actor: AuthUser, dto: BindFactoryGatewayDto) {
    if (!actor.tenantId) throw new BadRequestException('当前账号未关联租户，不能绑定网关');
    const gateway = await this.resolveBindableGateway(dto);
    const onlineThresholdMs = Number(process.env.DEVICE_OFFLINE_TIMEOUT_SECONDS ?? 90) * 1000;
    const online = Boolean(gateway.lastSeenAt && Date.now() - gateway.lastSeenAt.getTime() <= onlineThresholdMs);
    return {
      sn: gateway.sn,
      batchNo: gateway.batchNo,
      producedAt: gateway.producedAt,
      firstSeenAt: gateway.firstSeenAt,
      lastSeenAt: gateway.lastSeenAt,
      online,
      suggestedName: `网关 ${gateway.sn}`,
    };
  }

  async resetBindCode(id: string) {
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!factoryGateway) throw new NotFoundException('出厂网关不存在');
    if (factoryGateway.status === 'BOUND' || factoryGateway.deviceId) throw new BadRequestException('该网关已绑定，不能重置绑定码');

    const bindCode = this.randomBindCode();
    const updated = await this.prisma.factoryGateway.update({
      where: { id },
      data: {
        bindCodeHash: await bcrypt.hash(bindCode, 10),
        bindCodeEncrypted: this.encryptCredential(bindCode, 'bind-code'),
      },
      include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
    });
    return { ...this.toPublicFactoryGateway(updated), bindCode };
  }

  async getBindCredential(id: string) {
    const factoryGateway = await this.requireUnboundGateway(id);
    if (!factoryGateway.bindCodeEncrypted) {
      throw new ConflictException('该网关使用旧版不可恢复绑定码，请确认重置后再查看二维码；旧二维码会在重置后失效');
    }
    return {
      ...this.toPublicFactoryGateway(factoryGateway),
      bindCode: this.decryptCredential(factoryGateway.bindCodeEncrypted, 'bind-code'),
    };
  }

  async prepareNfcCredential(id: string) {
    const factoryGateway = await this.requireUnboundGateway(id);
    if (factoryGateway.nfcTokenEncrypted) {
      return this.toNfcCredential(
        factoryGateway,
        this.decryptCredential(factoryGateway.nfcTokenEncrypted, 'nfc-token'),
      );
    }
    if (factoryGateway.nfcTokenHash || factoryGateway.nfcIssuedAt) {
      throw new ConflictException('该网关已有旧版 NFC 凭证但无法恢复；如需重写标签，请确认重置 NFC 凭证，旧标签会失效');
    }

    return this.issueNfcCredential(factoryGateway);
  }

  async resetNfcCredential(id: string) {
    const factoryGateway = await this.requireUnboundGateway(id);
    return this.issueNfcCredential(factoryGateway);
  }

  private async issueNfcCredential(factoryGateway: Prisma.FactoryGatewayGetPayload<Record<string, never>>) {
    const nfcToken = this.randomSecret(24);
    const issuedAt = new Date();
    const updated = await this.prisma.factoryGateway.update({
      where: { id: factoryGateway.id },
      data: {
        nfcTokenHash: await bcrypt.hash(nfcToken, 10),
        nfcTokenEncrypted: this.encryptCredential(nfcToken, 'nfc-token'),
        nfcIssuedAt: issuedAt,
      },
      include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
    });
    return this.toNfcCredential(updated, nfcToken);
  }

  private toNfcCredential<T extends {
    id: string;
    sn: string;
    deviceSecretHash?: string;
    bindCodeHash?: string;
    bindCodeEncrypted?: string | null;
    nfcTokenHash?: string | null;
    nfcTokenEncrypted?: string | null;
    nfcIssuedAt?: Date | null;
  }>(gateway: T, nfcToken: string) {
    const payload = `weikong://gateway-bind?sn=${encodeURIComponent(gateway.sn)}&nfcToken=${encodeURIComponent(nfcToken)}`;
    return { ...this.toPublicFactoryGateway(gateway), nfcToken, payload };
  }

  async backfillIdentity(id: string, dto: BackfillGatewayIdentityDto) {
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!gateway || gateway.status === 'REMOVED') throw new NotFoundException('出厂网关不存在');
    if (gateway.hardwareId !== this.normalizeHardwareId(dto.hardwareId)) throw new BadRequestException('HardwareId 与出厂档案不一致');
    const chipUsid = this.normalizeChipUsid(dto.chipUsid);
    if (gateway.chipUsid && gateway.chipUsid !== chipUsid) throw new BadRequestException('已登记芯片身份不能修改，请走设备更换流程');
    const owner = await this.prisma.factoryGateway.findUnique({ where: { chipUsid } });
    if (owner && owner.id !== id) throw new ConflictException('该芯片已登记到其他网关');
    if (dto.deviceSecret && !await bcrypt.compare(dto.deviceSecret, gateway.deviceSecretHash)) throw new BadRequestException('原设备密钥校验失败，未修改任何连接凭据');
    const updated = await this.prisma.factoryGateway.update({
      where: { id, deviceSecretHash: gateway.deviceSecretHash, AND: [{ chipUsid: gateway.chipUsid }], status: gateway.status },
      data: { chipUsid, ...(dto.deviceSecret ? { activationSecretEncrypted: this.encryptCredential(dto.deviceSecret, 'activation-secret') } : {}) },
    });
    return this.toPublicFactoryGateway(updated);
  }

  async resetActivation(id: string) {
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { id }, include: { product: true } });
    if (!factoryGateway) throw new NotFoundException('出厂网关不存在');
    if (factoryGateway.status === 'REMOVED' || factoryGateway.status === 'DISABLED') throw new BadRequestException('该网关已移除或停用，不能签发授权');
	if (!factoryGateway.chipUsid) {
	  throw new BadRequestException('旧网关尚未登记 RJGT102 芯片 USID，请先完成芯片身份补录');
	}

    if (!factoryGateway.activationSecretEncrypted) {
      throw new BadRequestException('旧设备缺少可恢复的设备凭据，请先补录芯片身份及原激活文件中的设备密钥；不会自动重置现有连接');
    }
    const deviceSecret = this.decryptCredential(factoryGateway.activationSecretEncrypted, 'activation-secret');
    const licenseSequence = factoryGateway.licenseSequence + 1;
    const activationFile = this.createActivationFile({ hardwareId: factoryGateway.hardwareId, chipUsid: factoryGateway.chipUsid, sn: factoryGateway.sn, deviceSecret, licenseSequence, product: factoryGateway.product });
    const updated = await this.prisma.factoryGateway.update({
      where: { id, licenseSequence: factoryGateway.licenseSequence, status: factoryGateway.status, deviceSecretHash: factoryGateway.deviceSecretHash },
      data: {
        licenseSequence,
        issuedLicense: this.publicLicenseMetadata(activationFile),
      },
      include: { device: { select: { id: true, name: true, deviceKey: true, tenantId: true, status: true } } },
    });
    return {
      ...this.toPublicFactoryGateway(updated),
      deviceSecret,
      activationFile,
    };
  }

  async removeUnbound(id: string) {
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!gateway || gateway.status === 'REMOVED') throw new NotFoundException('出厂网关不存在');
    if (gateway.deviceId || gateway.status === 'BOUND') {
      throw new BadRequestException('该网关已绑定，请先执行解除绑定/回收网关');
    }
    const updated = await this.prisma.factoryGateway.update({
      where: { id },
      data: {
        status: 'REMOVED',
        nfcTokenHash: null,
        nfcTokenEncrypted: null,
        nfcIssuedAt: null,
        deviceSecretHash: await bcrypt.hash(this.randomSecret(24), 10),
      },
    });
    return { id: updated.id, sn: updated.sn, hardwareId: updated.hardwareId, status: updated.status };
  }

  async reclaim(id: string) {
    return this.prisma.$transaction(async (tx) => {
      const gateway = await tx.factoryGateway.findUnique({
        where: { id },
        include: { device: { select: { id: true, name: true, tenantId: true } } },
      });
      if (!gateway || gateway.status === 'REMOVED') throw new NotFoundException('出厂网关不存在');
      if (!gateway.deviceId || !gateway.device) throw new BadRequestException('该网关尚未绑定，无需回收');
      const device = gateway.device;
      // Reclamation must not delete tenant-owned children or transfer them to a new tenant.
      // Explicitly detach before deleting the gateway, and clear stale online state.
      const releasedChildren = await tx.device.updateMany({
        where: { gatewayId: device.id },
        data: { gatewayId: null, status: 'OFFLINE' },
      });
      await tx.factoryGateway.update({
        where: { id },
        data: { status: 'UNBOUND', deviceId: null, boundAt: null },
      });
      await tx.device.delete({ where: { id: device.id } });
      return { id: gateway.id, sn: gateway.sn, hardwareId: gateway.hardwareId, device, releasedChildrenCount: releasedChildren.count };
    });
  }

  async markFactoryOnline(hardwareId: string) {
    const normalized = this.normalizeHardwareId(hardwareId);
    const now = new Date();
    const existing = await this.prisma.factoryGateway.findUnique({ where: { hardwareId: normalized } });
    if (!existing) throw new NotFoundException('出厂网关尚未登记');
    const gateway = await this.prisma.factoryGateway.update({
      where: { hardwareId: normalized },
      data: {
        ...(!existing.firstSeenAt ? { firstSeenAt: now } : {}),
        lastSeenAt: now,
      },
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

  private normalizeChipUsid(value: string) {
	const chipUsid = value.trim().toUpperCase();
	if (!/^[0-9A-F]{16}$/.test(chipUsid)) {
	  throw new BadRequestException('加密芯片 USID 必须是完整的 16 位十六进制字符');
	}
	return chipUsid;
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

  private async requireUnboundGateway(id: string) {
    const factoryGateway = await this.prisma.factoryGateway.findUnique({ where: { id } });
    if (!factoryGateway || factoryGateway.status === 'REMOVED') throw new NotFoundException('出厂网关不存在');
    if (factoryGateway.status === 'BOUND' || factoryGateway.deviceId) {
      throw new BadRequestException('该网关已经绑定，不能查看或更新绑定凭证');
    }
    return factoryGateway;
  }

  private credentialKey() {
    const configured = this.config.get<string>('FACTORY_CREDENTIAL_ENCRYPTION_KEY')?.trim();
    const fallback = this.config.get<string>('JWT_SECRET', 'weikong_iot_secret');
    return createHash('sha256').update(configured || fallback).digest();
  }

  private encryptCredential(value: string, purpose: 'bind-code' | 'nfc-token' | 'activation-secret') {
    if (purpose === 'activation-secret' && !this.config.get<string>('FACTORY_CREDENTIAL_ENCRYPTION_KEY')?.trim()) throw new InternalServerErrorException('设备凭据加密密钥未配置，不能签发或恢复授权');
    const iv = randomBytes(12);
    const cipher = createCipheriv('aes-256-gcm', this.credentialKey(), iv);
    cipher.setAAD(Buffer.from(`factory-gateway:${purpose}:v1`));
    const encrypted = Buffer.concat([cipher.update(value, 'utf8'), cipher.final()]);
    const tag = cipher.getAuthTag();
    return `v1.${iv.toString('base64url')}.${tag.toString('base64url')}.${encrypted.toString('base64url')}`;
  }

  private decryptCredential(payload: string, purpose: 'bind-code' | 'nfc-token' | 'activation-secret') {
    if (purpose === 'activation-secret' && !this.config.get<string>('FACTORY_CREDENTIAL_ENCRYPTION_KEY')?.trim()) throw new InternalServerErrorException('设备凭据加密密钥未配置，不能恢复授权');
    try {
      const [version, ivValue, tagValue, encryptedValue] = payload.split('.');
      if (version !== 'v1' || !ivValue || !tagValue || !encryptedValue) throw new Error('invalid credential payload');
      const decipher = createDecipheriv('aes-256-gcm', this.credentialKey(), Buffer.from(ivValue, 'base64url'));
      decipher.setAAD(Buffer.from(`factory-gateway:${purpose}:v1`));
      decipher.setAuthTag(Buffer.from(tagValue, 'base64url'));
      return Buffer.concat([
        decipher.update(Buffer.from(encryptedValue, 'base64url')),
        decipher.final(),
      ]).toString('utf8');
    } catch {
      throw new InternalServerErrorException('绑定凭证解密失败，请检查服务端凭证加密密钥是否发生变化');
    }
  }

  private getPublicMqttBroker() {
    const host = process.env.MQTT_PUBLIC_HOST ?? '127.0.0.1';
    const port = Number(process.env.MQTT_PUBLIC_PORT ?? 1883);
    return `tcp://${host}:${port}`;
  }

  private createActivationFile(input: { hardwareId: string; chipUsid?: string | null; sn: string; deviceSecret: string; licenseSequence: number; product: LicenseProductProfile }) {
	const issuedAt = new Date();
	// A signed empty expiresAt means perpetual; positive values retain timed licenses.
	const licenseDays = Number(this.config.get<string>('ACTIVATION_LICENSE_DAYS', '0'));
	if (!Number.isSafeInteger(licenseDays) || licenseDays < 0 || licenseDays > 365000) {
	  throw new InternalServerErrorException('授权有效期配置无效：天数必须为非负整数，0 表示永久有效');
	}
	const expiresAt = licenseDays === 0 ? '' : new Date(issuedAt.getTime() + licenseDays * 24 * 60 * 60 * 1000).toISOString();
	const features = ['collection', 'forward', 'history', 'remote-storage', 'edge', 'alarms', 'cloud', 'remote-access', 'remote-ssh'];
	const hardwareProfile = this.licenseHardwareProfile(input.product);
	const payload = {
	  enabled: true,
	  licenseVersion: 2,
	  keyId: this.config.get<string>('ACTIVATION_SIGNING_KEY_ID', '').trim(),
	  licenseSequence: input.licenseSequence,
	  hardwareId: input.hardwareId.trim().toUpperCase(),
	  chipUsid: input.chipUsid?.trim().toUpperCase() || '',
	  sn: input.sn.trim(),
	  deviceSecret: input.deviceSecret,
	  broker: this.getPublicMqttBroker(),
	  cloudApi: this.config.get<string>('ACTIVATION_CLOUD_API', '').trim(),
	  remoteAccessEndpoint: this.config.get<string>('ACTIVATION_REMOTE_ACCESS_ENDPOINT', '').trim(),
	  issuedAt: issuedAt.toISOString(),
	  expiresAt,
	  features,
	  productModel: input.product.code,
	  hardwareProfile,
	  nonce: randomBytes(18).toString('base64url'),
	};
	const privateKeyValue = this.config.get<string>('ACTIVATION_SIGNING_PRIVATE_KEY', '').trim();
	if (!privateKeyValue || !/^[A-Za-z0-9._-]{1,64}$/.test(payload.keyId)) {
	  throw new InternalServerErrorException('授权签名私钥未配置，不能生成设备授权文件');
	}
	try {
	  const privateKey = createPrivateKey({
		key: Buffer.from(privateKeyValue, 'base64'),
		format: 'der',
		type: 'pkcs8',
	  });
	  if (privateKey.asymmetricKeyType !== 'ed25519') throw new Error('not Ed25519');
	  const signature = sign(null, Buffer.from(JSON.stringify(payload), 'utf8'), privateKey).toString('base64');
	  return { ...payload, signature };
	} catch {
	  throw new InternalServerErrorException('授权签名私钥无效，不能生成设备授权文件');
	}
  }

  private licenseHardwareProfile(product: LicenseProductProfile): LicenseHardwareProfile {
    const missing: string[] = [];
    const countFields = [
      ['网口数量', product.ethernetPortCount],
      ['RS232 数量', product.rs232PortCount],
      ['RS485 数量', product.rs485PortCount],
      ['CAN 数量', product.canPortCount],
    ] as const;
    for (const [label, value] of countFields) {
      if (!Number.isInteger(value) || Number(value) < 0 || Number(value) > 64) missing.push(label);
    }
    if (typeof product.supports4G !== 'boolean') missing.push('4G 支持情况');
    if (typeof product.supportsWifi !== 'boolean') missing.push('Wi-Fi 支持情况');
    if (missing.length > 0) {
      throw new BadRequestException(`产品型号 ${product.code} 的硬件规格不完整，请先补充：${missing.join('、')}`);
    }
    return {
      ethernetPortCount: product.ethernetPortCount!,
      rs232PortCount: product.rs232PortCount!,
      rs485PortCount: product.rs485PortCount!,
      canPortCount: product.canPortCount!,
      supports4G: product.supports4G!,
      supportsWifi: product.supportsWifi!,
    };
  }

  private async resolveBindableGateway(dto: BindFactoryGatewayDto) {
    const sn = dto.sn.trim().toUpperCase();
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { sn } });
    if (!gateway) throw new NotFoundException('未找到该 SN，请核对网关铭牌或交付单');
    if (gateway.status === 'REMOVED') throw new BadRequestException('该网关已被移除，请联系设备供应方重新登记');
    if (gateway.status === 'DISABLED') throw new BadRequestException('该网关已被停用，请联系设备供应方');
    if (gateway.status === 'BOUND' || gateway.deviceId) throw new BadRequestException('该网关已绑定，不能重复绑定');
    const bindCode = dto.bindCode?.trim();
    const nfcToken = dto.nfcToken?.trim();
    if (!bindCode && !nfcToken) throw new BadRequestException('请提供 6 位绑定码或有效的 NFC 凭证');
    const bindCodeValid = Boolean(bindCode && await bcrypt.compare(bindCode, gateway.bindCodeHash));
    const nfcTokenValid = Boolean(nfcToken && gateway.nfcTokenHash && await bcrypt.compare(nfcToken, gateway.nfcTokenHash));
    if (!bindCodeValid && !nfcTokenValid) {
      throw new BadRequestException(nfcToken ? 'NFC 凭证无效或已被重新写入，请联系管理员重写标签' : '绑定码不正确，请核对交付单或联系设备供应方');
    }
    return gateway;
  }

  private publicLicenseMetadata(activation: {signature:string;licenseSequence:number;licenseVersion:number;keyId:string;issuedAt:string;expiresAt:string;features:string[];productModel:string;hardwareProfile:LicenseHardwareProfile}) {
    return {licenseId:createHash('sha256').update(activation.signature).digest('hex'),licenseSequence:activation.licenseSequence,licenseVersion:activation.licenseVersion,keyId:activation.keyId,issuedAt:activation.issuedAt,expiresAt:activation.expiresAt,features:activation.features,productModel:activation.productModel,hardwareProfile:activation.hardwareProfile};
  }

  private toPublicFactoryGateway<T extends LicenseStatusInput & {
    deviceSecretHash?: string;
    activationSecretEncrypted?: string | null;
    bindCodeHash?: string;
    bindCodeEncrypted?: string | null;
    nfcTokenHash?: string | null;
    nfcTokenEncrypted?: string | null;
    nfcIssuedAt?: Date | null;
  }>(gateway: T) {
    const bindCredentialRecoverable = Boolean(gateway.bindCodeEncrypted);
    const nfcCredentialRecoverable = Boolean(gateway.nfcTokenEncrypted);
    const nfcCredentialIssued = Boolean(gateway.nfcTokenHash || gateway.nfcIssuedAt);
    const {
      deviceSecretHash: _deviceSecretHash,
      activationSecretEncrypted: _activationSecretEncrypted,
      bindCodeHash: _bindCodeHash,
      bindCodeEncrypted: _bindCodeEncrypted,
      nfcTokenHash: _nfcTokenHash,
      nfcTokenEncrypted: _nfcTokenEncrypted,
      ...safe
    } = gateway;
    return { ...safe, ...authorizationStatus(gateway), bindCredentialRecoverable, nfcCredentialRecoverable, nfcCredentialIssued };
  }
}

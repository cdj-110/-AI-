import { BadRequestException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { randomBytes } from 'crypto';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';

interface ReportedTopologyPayload {
  gatewayKey?: string;
  configVersion?: string;
  updatedAt?: string;
  devices?: unknown;
}

interface ReportedDevice {
  deviceKey: string;
  name?: string;
  protocol?: string;
  address?: string;
  slaveId?: number;
  pointCount?: number;
  metrics?: ReportedMetric[];
}

interface ReportedMetric {
  identifier: string;
  name?: string;
  dataType?: string;
  unit?: string;
}

interface DraftItem extends ReportedDevice {
  status: 'NEW' | 'UPDATE' | 'UNCHANGED' | 'CONFLICT' | 'MISSING_UNREPORTED';
  existingDeviceId?: string;
  currentGatewayId?: string | null;
  currentGatewayKey?: string | null;
  conflictReason?: string;
}

@Injectable()
export class GatewayTopologyService {
  constructor(private readonly prisma: PrismaService) {}

  async report(payload: ReportedTopologyPayload) {
    const gatewayKey = String(payload.gatewayKey ?? '').trim();
    if (!gatewayKey) throw new BadRequestException('gatewayKey is required');
    const gateway = await this.prisma.device.findFirst({
      where: { deviceKey: gatewayKey, deviceType: 'GATEWAY' },
      include: { children: true },
    });
    if (!gateway) throw new NotFoundException('gateway device not found');

    const reportedDevices = this.normalizeReportedDevices(payload.devices);
    const deviceKeys = [...new Set(reportedDevices.map((item) => item.deviceKey))];
    const existingDevices = deviceKeys.length
      ? await this.prisma.device.findMany({
          where: { deviceKey: { in: deviceKeys } },
          include: { gateway: { select: { id: true, deviceKey: true } } },
        })
      : [];
    const existingByKey = new Map(existingDevices.map((device) => [device.deviceKey, device]));
    const reportedKeySet = new Set(deviceKeys);
    const currentChildrenByKey = new Map(gateway.children.map((child) => [child.deviceKey, child]));
    const items: DraftItem[] = [];
    const conflicts: Array<{ deviceKey: string; reason: string }> = [];

    for (const reported of reportedDevices) {
      const existing = existingByKey.get(reported.deviceKey);
      if (!existing) {
        items.push({ ...reported, status: 'NEW' });
        continue;
      }
      if (existing.id === gateway.id) {
        const reason = 'deviceKey matches gateway itself';
        items.push({ ...reported, status: 'CONFLICT', existingDeviceId: existing.id, conflictReason: reason });
        conflicts.push({ deviceKey: reported.deviceKey, reason });
        continue;
      }
      if (existing.tenantId !== gateway.tenantId) {
        const reason = 'deviceKey belongs to another tenant';
        items.push({ ...reported, status: 'CONFLICT', existingDeviceId: existing.id, currentGatewayId: existing.gatewayId, currentGatewayKey: existing.gateway?.deviceKey, conflictReason: reason });
        conflicts.push({ deviceKey: reported.deviceKey, reason });
        continue;
      }
      if (existing.gatewayId && existing.gatewayId !== gateway.id) {
        const reason = 'deviceKey belongs to another gateway';
        items.push({ ...reported, status: 'CONFLICT', existingDeviceId: existing.id, currentGatewayId: existing.gatewayId, currentGatewayKey: existing.gateway?.deviceKey, conflictReason: reason });
        conflicts.push({ deviceKey: reported.deviceKey, reason });
        continue;
      }
      if (existing.deviceType !== 'GATEWAY_CHILD' && existing.deviceType !== 'DIRECT') {
        const reason = `device type ${existing.deviceType} cannot become gateway child`;
        items.push({ ...reported, status: 'CONFLICT', existingDeviceId: existing.id, currentGatewayId: existing.gatewayId, currentGatewayKey: existing.gateway?.deviceKey, conflictReason: reason });
        conflicts.push({ deviceKey: reported.deviceKey, reason });
        continue;
      }
      items.push({
        ...reported,
        status: this.hasTopologyChange(existing, reported, gateway.id) ? 'UPDATE' : 'UNCHANGED',
        existingDeviceId: existing.id,
        currentGatewayId: existing.gatewayId,
        currentGatewayKey: existing.gateway?.deviceKey,
      });
    }

    for (const child of gateway.children) {
      if (reportedKeySet.has(child.deviceKey)) continue;
      items.push({
        deviceKey: child.deviceKey,
        name: child.name,
        protocol: child.protocol,
        status: 'MISSING_UNREPORTED',
        existingDeviceId: child.id,
        currentGatewayId: gateway.id,
        currentGatewayKey: gateway.deviceKey,
      });
    }

    const hasPendingChange = items.some((item) => ['NEW', 'UPDATE', 'MISSING_UNREPORTED'].includes(item.status));
    const status = conflicts.length ? 'CONFLICT' : hasPendingChange ? 'PENDING' : 'CONFIRMED';
    return this.prisma.gatewayTopologyDraft.upsert({
      where: { gatewayDeviceId: gateway.id },
      create: {
        gatewayDeviceId: gateway.id,
        gatewayKey,
        reportedConfigVersion: payload.configVersion ? String(payload.configVersion) : undefined,
        items: items as unknown as Prisma.InputJsonValue,
        conflicts: conflicts.length ? conflicts as unknown as Prisma.InputJsonValue : undefined,
        status,
        reportedAt: this.parseDate(payload.updatedAt) ?? new Date(),
      },
      update: {
        gatewayKey,
        reportedConfigVersion: payload.configVersion ? String(payload.configVersion) : null,
        items: items as unknown as Prisma.InputJsonValue,
        conflicts: conflicts.length ? conflicts as unknown as Prisma.InputJsonValue : Prisma.DbNull,
        status,
        reportedAt: this.parseDate(payload.updatedAt) ?? new Date(),
        confirmedAt: null,
        confirmedBy: null,
        ignoredAt: null,
        ignoredBy: null,
      },
    });
  }

  async findDraft(actor: AuthUser, gatewayId: string) {
    await this.getAccessibleGateway(actor, gatewayId);
    return this.prisma.gatewayTopologyDraft.findUnique({ where: { gatewayDeviceId: gatewayId } });
  }

  async confirm(actor: AuthUser, gatewayId: string) {
    const gateway = await this.getAccessibleGateway(actor, gatewayId);
    const draft = await this.prisma.gatewayTopologyDraft.findUnique({ where: { gatewayDeviceId: gateway.id } });
    if (!draft) throw new NotFoundException('topology draft not found');
    const items = this.parseDraftItems(draft.items);
    const conflicts = items.filter((item) => item.status === 'CONFLICT');
    if (conflicts.length) throw new BadRequestException('topology draft has conflicts');

    const applied = { created: 0, updated: 0, unchanged: 0, skipped: 0 };
    await this.prisma.$transaction(async (tx) => {
      for (const item of items) {
        if (!['NEW', 'UPDATE', 'UNCHANGED'].includes(item.status)) {
          applied.skipped += 1;
          continue;
        }
        const existing = await tx.device.findUnique({ where: { deviceKey: item.deviceKey } });
        if (!existing) {
          const credentials = await this.generateMqttCredentials(item.deviceKey);
          await tx.device.create({
            data: {
              tenantId: gateway.tenantId,
              deviceKey: item.deviceKey,
              name: item.name || item.deviceKey,
              protocol: item.protocol || 'MODBUS',
              deviceType: 'GATEWAY_CHILD',
              gatewayId: gateway.id,
              status: 'OFFLINE',
              mqttClientId: credentials.clientId,
              mqttUsername: credentials.username,
              mqttPasswordHash: credentials.passwordHash,
              mqttPasswordUpdatedAt: new Date(),
            },
          });
          applied.created += 1;
          continue;
        }
        if (existing.tenantId !== gateway.tenantId) throw new BadRequestException(`deviceKey ${item.deviceKey} belongs to another tenant`);
        if (existing.gatewayId && existing.gatewayId !== gateway.id) throw new BadRequestException(`deviceKey ${item.deviceKey} belongs to another gateway`);
        await tx.device.update({
          where: { id: existing.id },
          data: {
            name: item.name || existing.name,
            protocol: item.protocol || existing.protocol,
            deviceType: 'GATEWAY_CHILD',
            gatewayId: gateway.id,
          },
        });
        if (item.status === 'UNCHANGED') applied.unchanged += 1;
        else applied.updated += 1;
      }
      await tx.gatewayTopologyDraft.update({
        where: { id: draft.id },
        data: {
          status: 'CONFIRMED',
          confirmedAt: new Date(),
          confirmedBy: actor.username,
        },
      });
    });
    return { ...applied, status: 'CONFIRMED' };
  }

  async ignore(actor: AuthUser, gatewayId: string) {
    await this.getAccessibleGateway(actor, gatewayId);
    const draft = await this.prisma.gatewayTopologyDraft.findUnique({ where: { gatewayDeviceId: gatewayId } });
    if (!draft) throw new NotFoundException('topology draft not found');
    return this.prisma.gatewayTopologyDraft.update({
      where: { id: draft.id },
      data: { status: 'IGNORED', ignoredAt: new Date(), ignoredBy: actor.username },
    });
  }

  private async getAccessibleGateway(actor: AuthUser, gatewayId: string) {
    const gateway = await this.prisma.device.findUnique({ where: { id: gatewayId } });
    if (!gateway) throw new NotFoundException('gateway device not found');
    if (gateway.deviceType !== 'GATEWAY') throw new BadRequestException('device is not a gateway');
    if (actor.role !== 'SUPER_ADMIN' && gateway.tenantId !== actor.tenantId) throw new ForbiddenException('no permission to access this gateway');
    return gateway;
  }

  private normalizeReportedDevices(value: unknown): ReportedDevice[] {
    if (!Array.isArray(value)) return [];
    const seen = new Set<string>();
    const devices: ReportedDevice[] = [];
    for (const item of value) {
      if (!item || typeof item !== 'object') continue;
      const raw = item as Record<string, unknown>;
      const deviceKey = String(raw.deviceKey ?? '').trim();
      if (!deviceKey || seen.has(deviceKey)) continue;
      seen.add(deviceKey);
      const metrics = Array.isArray(raw.metrics)
        ? raw.metrics
            .map((metric) => this.normalizeMetric(metric))
            .filter((metric): metric is ReportedMetric => Boolean(metric))
        : [];
      devices.push({
        deviceKey,
        name: this.optionalString(raw.name),
        protocol: this.optionalString(raw.protocol),
        address: this.optionalString(raw.address),
        slaveId: typeof raw.slaveId === 'number' ? raw.slaveId : undefined,
        pointCount: typeof raw.pointCount === 'number' ? raw.pointCount : metrics.length,
        metrics,
      });
    }
    return devices;
  }

  private normalizeMetric(value: unknown): ReportedMetric | null {
    if (!value || typeof value !== 'object') return null;
    const raw = value as Record<string, unknown>;
    const identifier = String(raw.identifier ?? '').trim();
    if (!identifier) return null;
    return {
      identifier,
      name: this.optionalString(raw.name),
      dataType: this.optionalString(raw.dataType),
      unit: this.optionalString(raw.unit),
    };
  }

  private optionalString(value: unknown) {
    return typeof value === 'string' && value.trim() ? value.trim() : undefined;
  }

  private hasTopologyChange(existing: { name: string; protocol: string; gatewayId: string | null }, reported: ReportedDevice, gatewayId: string) {
    return existing.gatewayId !== gatewayId
      || Boolean(reported.name && reported.name !== existing.name)
      || Boolean(reported.protocol && reported.protocol !== existing.protocol);
  }

  private parseDraftItems(value: Prisma.JsonValue): DraftItem[] {
    return Array.isArray(value) ? value as unknown as DraftItem[] : [];
  }

  private parseDate(value: unknown) {
    if (typeof value !== 'string' && typeof value !== 'number') return undefined;
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? undefined : date;
  }

  private generateMqttPassword() {
    return randomBytes(18).toString('base64url');
  }

  private async generateMqttCredentials(deviceKey: string) {
    const safeKey = deviceKey.replace(/[^a-zA-Z0-9_-]/g, '_');
    const suffix = randomBytes(4).toString('hex');
    const password = this.generateMqttPassword();
    return {
      clientId: `wk_${safeKey}_${suffix}`,
      username: `device:${safeKey}_${suffix}`,
      passwordHash: await bcrypt.hash(password, 10),
    };
  }
}

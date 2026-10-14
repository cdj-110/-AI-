import { BadRequestException, ConflictException, Injectable, NotFoundException, OnModuleDestroy, OnModuleInit, ServiceUnavailableException, UnauthorizedException } from '@nestjs/common';
import { createHmac, randomUUID, timingSafeEqual } from 'node:crypto';
import { createReadStream, createWriteStream, promises as fs } from 'node:fs';
import { join } from 'node:path';
import { pipeline } from 'node:stream/promises';
import { Transform } from 'node:stream';
import { Request, Response } from 'express';
import mqtt, { MqttClient } from 'mqtt';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { MAX_OTA_SIZE, verifyOtaPackage } from './ota-package';
import { assertForwardOtaRelease } from './ota-release';
import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';

const storage = () => process.env.OTA_PACKAGE_DIR || '/var/lib/weikong/ota';
const maxUpload = MAX_OTA_SIZE + 4096 + 8;

@Injectable()
export class OtaService implements OnModuleInit, OnModuleDestroy {
  private mqtt?: MqttClient;
  private ready = false;
  private timer?: ReturnType<typeof setInterval>;
  constructor(private readonly prisma: PrismaService) {}

  onModuleInit() {
    this.mqtt = mqtt.connect(process.env.MQTT_URL ?? 'mqtt://localhost:1883', {
      clientId: `weikong-api-ota-${Date.now()}`, username: process.env.MQTT_INGEST_USERNAME ?? 'platform-ingest',
      password: process.env.MQTT_INGEST_PASSWORD ?? 'platform-ingest-secret', reconnectPeriod: 3000, connectTimeout: 5000,
    });
    this.mqtt.on('connect', () => this.mqtt?.subscribe('weikong/factory/+/ota/reply', { qos: 1 }, (error, granted) => {
      this.ready = !error && granted?.length === 1 && granted[0].qos !== 128;
    }));
    this.mqtt.on('close', () => { this.ready = false; });
    this.mqtt.on('error', error => console.error('[ota] MQTT', error.message));
    this.mqtt.on('message', (topic, payload, packet) => { if (!packet.retain) void this.handleReply(topic, payload).catch(error => console.error('[ota] receipt', error)); });
    this.timer = setInterval(() => { void this.expireTasks(); }, 15000);
  }

  onModuleDestroy() { if (this.timer) clearInterval(this.timer); this.mqtt?.end(true); }

  private trustedKeys(): Record<string, string> {
    try {
      const keys = JSON.parse(process.env.OTA_TRUSTED_PUBLIC_KEYS || '{}') as Record<string, string>;
      if (!keys || typeof keys !== 'object' || !Object.keys(keys).length) throw new Error();
      return keys;
    } catch { throw new ServiceUnavailableException('OTA 公钥尚未配置'); }
  }

  private downloadSecret(): string {
    const secret = process.env.OTA_DOWNLOAD_SECRET || '';
    if (secret.length < 32) throw new ServiceUnavailableException('OTA 下载令牌密钥尚未配置');
    return secret;
  }

  private publicOrigin(): string {
    const value = process.env.OTA_PUBLIC_BASE_URL || '';
    let parsed: URL;
    try { parsed = new URL(value); } catch { throw new ServiceUnavailableException('OTA 公网 HTTPS 地址尚未配置'); }
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash) throw new ServiceUnavailableException('OTA 公网地址必须使用 HTTPS');
    let licensedCloud: URL;
    try { licensedCloud = new URL(process.env.ACTIVATION_CLOUD_API || ''); } catch { throw new ServiceUnavailableException('授权云平台 HTTPS 地址尚未配置'); }
    if (licensedCloud.protocol !== 'https:' || licensedCloud.host.toLowerCase() !== parsed.host.toLowerCase()) throw new ServiceUnavailableException('授权文件中的云平台地址需改为同域名 HTTPS 后才能下发 OTA');
    return parsed.origin;
  }

  readiness() {
    try { this.trustedKeys(); this.downloadSecret(); this.publicOrigin(); return { enabled: true, reason: '' }; }
    catch (error) { return { enabled: false, reason: error instanceof Error ? error.message : 'OTA 尚未就绪' }; }
  }

  private packageView<T extends { release: bigint }>(item: T): Omit<T, 'release'> & { release: string } {
    return { ...item, release: item.release.toString() };
  }

  private async packageUsage() {
    const [currentTasks, activeTasks] = await Promise.all([
      this.prisma.otaTask.findMany({
        where: { status: 'SUCCEEDED' },
        select: { gatewayId: true, packageId: true },
        distinct: ['gatewayId'],
        orderBy: [{ gatewayId: 'asc' }, { completedAt: 'desc' }, { createdAt: 'desc' }],
      }),
      this.prisma.otaTask.groupBy({
        by: ['packageId'],
        where: { status: { in: ['PENDING', 'SENT', 'INSTALLING'] }, expiresAt: { gt: new Date() } },
        _count: { _all: true },
      }),
    ]);
    const current = new Map<string, number>();
    for (const task of currentTasks) current.set(task.packageId, (current.get(task.packageId) || 0) + 1);
    const active = new Map(activeTasks.map(item => [item.packageId, item._count._all]));
    return { current, active };
  }

  async listPackages() {
    const [items, usage] = await Promise.all([
      this.prisma.otaPackage.findMany({
        include: { _count: { select: { tasks: true } } },
        orderBy: { createdAt: 'desc' },
        take: 200,
      }),
      this.packageUsage(),
    ]);
    return items.map(item => {
      const currentDeviceCount = usage.current.get(item.id) || 0;
      const activeTaskCount = usage.active.get(item.id) || 0;
      return this.packageView({ ...item, currentDeviceCount, activeTaskCount, canPurge: item.fileAvailable && currentDeviceCount === 0 && activeTaskCount === 0 });
    });
  }

  async purgePackage(packageId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(packageId)) throw new BadRequestException('升级包编号无效');
    await this.expireTasks();
    const [pack, usage] = await Promise.all([this.prisma.otaPackage.findUnique({ where: { id: packageId } }), this.packageUsage()]);
    if (!pack) throw new NotFoundException('升级包不存在');
    if (!pack.fileAvailable) return this.packageView(pack);
    const currentDeviceCount = usage.current.get(pack.id) || 0;
    const activeTaskCount = usage.active.get(pack.id) || 0;
    if (currentDeviceCount > 0) throw new ConflictException(`仍有 ${currentDeviceCount} 台设备正在使用该版本，不能清理`);
    if (activeTaskCount > 0) throw new ConflictException('该升级包存在进行中的升级任务，不能清理');
    const updated = await this.prisma.otaPackage.update({ where: { id: packageId }, data: { fileAvailable: false, purgedAt: new Date() } });
    if (/^[0-9a-f-]{36}\.wku$/i.test(pack.fileName)) {
      try { await fs.rm(join(storage(), pack.fileName), { force: true }); }
      catch (error) {
        await this.prisma.otaPackage.update({ where: { id: packageId }, data: { fileAvailable: true, purgedAt: null } });
        throw new ServiceUnavailableException('升级包文件清理失败，请稍后重试');
      }
    }
    return this.packageView(updated);
  }

  async upload(request: Request, actor: AuthUser) {
    if (request.headers['content-type']?.split(';')[0] !== 'application/octet-stream') throw new BadRequestException('请上传 .wku 升级包');
    const keys = this.trustedKeys();
    await fs.mkdir(storage(), { recursive: true, mode: 0o700 });
    const fileName = `${randomUUID()}.wku`;
    const temp = join(storage(), `${fileName}.upload`);
    const final = join(storage(), fileName);
    let count = 0;
    let installedPath = '';
    try {
      await pipeline(request, new Transform({ transform(chunk: Buffer, _, callback) {
        count += chunk.length;
        callback(count > maxUpload ? new Error('升级包超过 64 MiB 限制') : null, chunk);
      } }), createWriteStream(temp, { flags: 'wx', mode: 0o600 }));
      const verified = await verifyOtaPackage(temp, keys);
      const existing = await this.prisma.otaPackage.findUnique({ where: { packageSha256: verified.packageSha256 } });
      if (existing?.fileAvailable) return this.packageView(existing);
      if (existing) {
        const restoredPath = join(storage(), existing.fileName);
        await fs.rename(temp, restoredPath);
        installedPath = restoredPath;
        const restored = await this.prisma.otaPackage.update({ where: { id: existing.id }, data: { fileAvailable: true, purgedAt: null } });
        return this.packageView(restored);
      }
      await fs.rename(temp, final);
      installedPath = final;
      const created = await this.prisma.otaPackage.create({ data: {
        model: verified.manifest.model, release: BigInt(verified.manifest.release), version: verified.manifest.version,
        keyId: verified.manifest.keyId, artifactSha256: verified.manifest.artifactSha256,
        packageSha256: verified.packageSha256, fileName, size: verified.size, createdBy: actor.sub,
      } });
      return this.packageView(created);
    } catch (error) {
      if (installedPath) await fs.rm(installedPath, { force: true });
      throw new BadRequestException(error instanceof Error ? error.message : '升级包上传失败');
    } finally { await fs.rm(temp, { force: true }); }
  }

  async listTasks(gatewayId: string) {
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { id: gatewayId }, select: { id: true } });
    if (!gateway) throw new NotFoundException('网关不存在');
    const items = await this.prisma.otaTask.findMany({ where: { gatewayId }, include: { package: true }, orderBy: { createdAt: 'desc' }, take: 30 });
    return items.map(item => ({ ...item, package: this.packageView(item.package) }));
  }

  async createTask(gatewayId: string, packageId: string, actor: AuthUser) {
    if (!/^[0-9a-f-]{36}$/i.test(gatewayId) || !/^[0-9a-f-]{36}$/i.test(packageId)) throw new BadRequestException('网关或升级包编号无效');
    const gateway = await this.prisma.factoryGateway.findUnique({
      where: { id: gatewayId },
      include: { device: { select: { cloudAccessEnabled: true, tenant: { select: cloudPolicyTenantSelect } } } },
    });
    const pack = await this.prisma.otaPackage.findUnique({ where: { id: packageId } });
    if (!gateway || !pack) throw new NotFoundException('网关或升级包不存在');
    if (!pack.fileAvailable) throw new BadRequestException('升级包文件已清理，请先在升级包管理中重新上传');
    if (gateway.model !== pack.model || gateway.status === 'DISABLED' || gateway.status === 'REMOVED') throw new BadRequestException('设备型号不匹配或网关已停用');
    if (gateway.device?.cloudAccessEnabled === false) throw new BadRequestException('该网关的云连接已被管理平台禁用');
    if (tenantBlocksCloud(gateway.device)) throw new BadRequestException('所属租户已停用设备云连接');
    const installed = await this.prisma.otaTask.findFirst({
      where: { gatewayId, status: 'SUCCEEDED' },
      include: { package: true },
      orderBy: { completedAt: 'desc' },
    });
    assertForwardOtaRelease(installed?.package.release, pack.release);
    if (!gateway.licenseActivatedAt || !gateway.lastSeenAt || Date.now() - gateway.lastSeenAt.getTime() > 120000) throw new BadRequestException('网关未激活或当前离线');
    await this.expireTasks();
    const active = await this.prisma.otaTask.count({ where: { gatewayId, status: { in: ['PENDING', 'SENT', 'INSTALLING'] }, expiresAt: { gt: new Date() } } });
    if (active) throw new ConflictException('该网关已有进行中的升级任务');
    const origin = this.publicOrigin();
    const secret = this.downloadSecret();
    if (!this.mqtt?.connected || !this.ready) throw new ServiceUnavailableException('OTA 下发通道暂未就绪');
    const expiresAt = new Date(Date.now() + 20 * 60_000);
    let task;
    try { task = await this.prisma.otaTask.create({ data: { gatewayId, packageId, createdBy: actor.sub, expiresAt } }); }
    catch (error) {
      if ((error as { code?: string }).code === 'P2002') throw new ConflictException('该网关已有进行中的升级任务');
      throw error;
    }
    const exp = Math.floor(expiresAt.getTime() / 1000);
    const sig = createHmac('sha256', secret).update(`${task.id}:${pack.id}:${exp}`).digest('hex');
    const packageUrl = `${origin}/api/ota/download/${pack.id}?taskId=${task.id}&exp=${exp}&sig=${sig}`;
    const payload = JSON.stringify({ taskId: task.id, hardwareId: gateway.hardwareId, sn: gateway.sn, packageUrl,
      packageSha256: pack.packageSha256, release: Number(pack.release), expiresAt: expiresAt.toISOString() });
    try {
      await new Promise<void>((resolve, reject) => this.mqtt!.publish(`weikong/factory/${gateway.hardwareId}/ota/set`, payload, { qos: 1, retain: false }, error => error ? reject(error) : resolve()));
      await this.prisma.otaTask.updateMany({ where: { id: task.id, status: 'PENDING' }, data: { status: 'SENT' } });
    } catch (error) {
      await this.prisma.otaTask.update({ where: { id: task.id }, data: { status: 'FAILED', message: error instanceof Error ? error.message : '发送失败', completedAt: new Date() } });
      throw new ServiceUnavailableException('OTA 任务发送失败');
    }
    return { ...task, status: 'SENT' };
  }

  async download(packageId: string, taskId: string, expText: string, sig: string, response: Response) {
    const exp = Number(expText);
    if (!Number.isSafeInteger(exp) || exp < Math.floor(Date.now() / 1000) || exp > Math.floor(Date.now() / 1000) + 21 * 60) throw new UnauthorizedException('下载令牌已失效');
    const expected = createHmac('sha256', this.downloadSecret()).update(`${taskId}:${packageId}:${exp}`).digest('hex');
    if (!/^[0-9a-f]{64}$/.test(sig) || !timingSafeEqual(Buffer.from(sig), Buffer.from(expected))) throw new UnauthorizedException('下载令牌无效');
    const task = await this.prisma.otaTask.findUnique({ where: { id: taskId }, include: { package: true, gateway: { include: { device: { select: { cloudAccessEnabled: true, tenant: { select: cloudPolicyTenantSelect } } } } } } });
    if (tenantBlocksCloud(task?.gateway.device) || task?.gateway.device?.cloudAccessEnabled === false) throw new UnauthorizedException('设备云连接已禁用');
    if (!task || task.packageId !== packageId || !task.package.fileAvailable || task.expiresAt.getTime() < Date.now() || !['PENDING', 'SENT', 'INSTALLING'].includes(task.status)) throw new UnauthorizedException('升级任务已失效');
    if (!/^[0-9a-f-]{36}\.wku$/i.test(task.package.fileName)) throw new NotFoundException('升级包路径无效');
    const path = join(storage(), task.package.fileName);
    const stat = await fs.stat(path).catch(() => null);
    if (!stat || stat.size !== task.package.size) throw new NotFoundException('升级包文件不可用');
    response.setHeader('Content-Type', 'application/octet-stream');
    response.setHeader('Content-Length', stat.size);
    response.setHeader('Cache-Control', 'no-store');
    createReadStream(path).pipe(response);
  }

  private async handleReply(topic: string, payload: Buffer) {
    const match = /^weikong\/factory\/([^/]+)\/ota\/reply$/.exec(topic);
    if (!match || payload.length > 4096) return;
    let reply: { taskId?: string; hardwareId?: string; release?: number; status?: string; message?: string };
    try { reply = JSON.parse(payload.toString()) as typeof reply; } catch { return; }
    if (!reply || !['INSTALLING', 'FAILED', 'SUCCEEDED', 'ROLLED_BACK'].includes(reply.status || '')) return;
    const task = await this.prisma.otaTask.findUnique({ where: { id: reply.taskId || '' }, include: { gateway: true, package: true } });
    if (!task || task.gateway.hardwareId !== match[1] || task.gateway.hardwareId !== reply.hardwareId || Number(task.package.release) !== reply.release) return;
    await this.prisma.otaTask.updateMany({ where: { id: task.id, status: { in: ['PENDING', 'SENT', 'INSTALLING', 'TIMEOUT'] } }, data: {
      status: reply.status, message: typeof reply.message === 'string' ? reply.message.slice(0, 500) : null,
      completedAt: reply.status === 'INSTALLING' ? null : new Date(),
    } });
  }

  private async expireTasks() {
    try { await this.prisma.otaTask.updateMany({ where: { status: { in: ['PENDING', 'SENT', 'INSTALLING'] }, expiresAt: { lt: new Date() } }, data: { status: 'TIMEOUT', message: '未在期限内收到网关最终回执，请检查设备状态', completedAt: new Date() } }); }
    catch (error) { console.error('[ota] expiry', error); }
  }
}

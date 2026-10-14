import { BadRequestException, ConflictException, Injectable, NotFoundException, OnModuleDestroy, OnModuleInit, ServiceUnavailableException } from '@nestjs/common';
import mqtt, { MqttClient } from 'mqtt';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { OperationLogsService } from '../operation-logs/operation-logs.service';
import { PrismaService } from '../prisma/prisma.service';
import { maintenanceGatewayInclude, maintenanceState } from './maintenance-state';

type MaintenanceAction = 'SERVICE_RESTART' | 'GATEWAY_REBOOT';

@Injectable()
export class GatewayMaintenanceService implements OnModuleInit, OnModuleDestroy {
  private client?: MqttClient;
  private ready = false;
  private timer?: ReturnType<typeof setInterval>;
  private retry?: ReturnType<typeof setTimeout>;

  constructor(private readonly prisma: PrismaService, private readonly logs: OperationLogsService) {}

  onModuleInit() {
    this.client = mqtt.connect(process.env.MQTT_URL ?? 'mqtt://localhost:1883', {
      clientId: `weikong-api-maintenance-${Date.now()}`,
      username: process.env.MQTT_INGEST_USERNAME ?? 'platform-ingest',
      password: process.env.MQTT_INGEST_PASSWORD ?? 'platform-ingest-secret',
      reconnectPeriod: 3000,
      connectTimeout: 5000,
    });
    this.client.on('connect', () => this.subscribe());
    this.client.on('close', () => { this.ready = false; });
    this.client.on('error', error => console.error('[gateway-maintenance] MQTT', error.message));
    this.client.on('message', (topic, payload, packet) => {
      if (!packet.retain) void this.handleReply(topic, payload).catch(error => console.error('[gateway-maintenance] reply', error.message));
    });
    this.timer = setInterval(() => { void this.expire().catch(error => console.error('[gateway-maintenance] expiry', error.message)); }, 5000);
  }

  onModuleDestroy() {
    if (this.timer) clearInterval(this.timer);
    if (this.retry) clearTimeout(this.retry);
    this.client?.end(true);
  }

  private subscribe() {
    if (this.retry) clearTimeout(this.retry);
    if (!this.client?.connected) return;
    this.client.subscribe('weikong/factory/+/maintenance/reply', { qos: 1 }, (error, granted) => {
      this.ready = !error && granted?.length === 1 && granted[0].qos !== 128;
      if (!this.ready) this.retry = setTimeout(() => this.subscribe(), 3000);
    });
  }

  async list(gatewayId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(gatewayId)) throw new BadRequestException('网关编号无效');
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { id: gatewayId }, select: { id: true } });
    if (!gateway) throw new NotFoundException('网关不存在');
    await this.expire();
    return this.prisma.gatewayMaintenanceTask.findMany({ where: { gatewayId }, orderBy: { createdAt: 'desc' }, take: 30 });
  }

  async status(gatewayId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(gatewayId)) throw new BadRequestException('网关编号无效');
    const gateway = await this.prisma.factoryGateway.findUnique({ where: { id: gatewayId }, include: maintenanceGatewayInclude });
    if (!gateway) throw new NotFoundException('网关不存在');
    return maintenanceState(gateway, Boolean(this.client?.connected && this.ready));
  }

  async create(gatewayId: string, action: MaintenanceAction, actor: AuthUser, request: { ip?: string; userAgent?: string | string[] }) {
    if (!/^[0-9a-f-]{36}$/i.test(gatewayId)) throw new BadRequestException('网关编号无效');
    if (!['SERVICE_RESTART', 'GATEWAY_REBOOT'].includes(action)) throw new BadRequestException('不支持的远程维护操作');
    const gateway = await this.prisma.factoryGateway.findUnique({
      where: { id: gatewayId },
      include: maintenanceGatewayInclude,
    });
    if (!gateway) throw new NotFoundException('网关不存在');
    const state = maintenanceState(gateway, true);
    if (!state.canMaintain) throw new BadRequestException(state.reason);
    await this.expire();
    const active = await this.prisma.gatewayMaintenanceTask.count({ where: { gatewayId, status: { in: ['PENDING', 'SENT'] }, expiresAt: { gt: new Date() } } });
    if (active) throw new ConflictException('该网关已有进行中的远程维护任务');
    if (!this.client?.connected || !this.ready) throw new ServiceUnavailableException('远程维护通道暂未就绪');

    const createdAt = new Date();
    const expiresAt = new Date(createdAt.getTime() + 60_000);
    const task = await this.prisma.gatewayMaintenanceTask.create({ data: { gatewayId, action, createdBy: actor.sub, createdAt, expiresAt } });
    const actionName = action === 'SERVICE_RESTART' ? '重启网关服务' : '重启网关';
    try {
      // Never send a destructive maintenance command without a durable audit record.
      await this.logs.record(actor, {
        module: '远程运维', action: actionName, targetType: 'GatewayMaintenanceTask', targetId: task.id, targetName: gateway.sn,
        ...request, detail: { gatewayId, hardwareId: gateway.hardwareId, sn: gateway.sn, action },
      });
    } catch {
      return this.prisma.gatewayMaintenanceTask.update({ where: { id: task.id }, data: { status: 'FAILED', message: '操作审计记录失败，指令未发送', completedAt: new Date() } });
    }

    const payload = JSON.stringify({ taskId: task.id, hardwareId: gateway.hardwareId, action, issuedAt: createdAt.toISOString(), expiresAt: expiresAt.toISOString() });
    try {
      await new Promise<void>((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('发送确认超时')), 5000);
        this.client!.publish(`weikong/factory/${gateway.hardwareId}/maintenance/set`, payload, { qos: 1, retain: false }, error => {
          clearTimeout(timer);
          error ? reject(error) : resolve();
        });
      });
      await this.prisma.gatewayMaintenanceTask.updateMany({ where: { id: task.id, status: 'PENDING' }, data: { status: 'SENT', message: '指令已下发，等待网关受理' } });
    } catch (error) {
      await this.prisma.gatewayMaintenanceTask.updateMany({ where: { id: task.id, status: 'PENDING' }, data: { status: 'FAILED', message: error instanceof Error ? error.message : '指令发送失败', completedAt: new Date() } });
      throw new ServiceUnavailableException('远程维护指令发送失败');
    }
    return this.prisma.gatewayMaintenanceTask.findUniqueOrThrow({ where: { id: task.id } });
  }

  async expire() {
    await this.prisma.gatewayMaintenanceTask.updateMany({
      where: { status: { in: ['PENDING', 'SENT'] }, expiresAt: { lte: new Date() } },
      data: { status: 'TIMEOUT', message: '未收到网关受理回执，请检查设备在线状态后再决定是否重试', completedAt: new Date() },
    });
  }

  private async handleReply(topic: string, payload: Buffer) {
    const match = /^weikong\/factory\/([^/]+)\/maintenance\/reply$/.exec(topic);
    if (!match || payload.length > 4096) return;
    let reply: { taskId?: string; hardwareId?: string; action?: string; status?: string; message?: string };
    try { reply = JSON.parse(payload.toString()) as typeof reply; } catch { return; }
    if (!reply || !['ACCEPTED', 'FAILED'].includes(reply.status || '') || !['SERVICE_RESTART', 'GATEWAY_REBOOT'].includes(reply.action || '')) return;
    const task = await this.prisma.gatewayMaintenanceTask.findUnique({ where: { id: reply.taskId || '' }, include: { gateway: true } });
    if (!task || task.gateway.hardwareId !== match[1] || task.gateway.hardwareId !== reply.hardwareId || task.action !== reply.action) return;
    await this.prisma.gatewayMaintenanceTask.updateMany({
      where: { id: task.id, status: { in: ['PENDING', 'SENT', 'TIMEOUT'] } },
      data: { status: reply.status, message: typeof reply.message === 'string' ? reply.message.slice(0, 500) : null, completedAt: new Date() },
    });
  }
}

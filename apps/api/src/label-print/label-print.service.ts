import { BadRequestException, Injectable, NotFoundException, UnauthorizedException } from '@nestjs/common';
import { createHash, randomBytes } from 'crypto';
import * as QRCode from 'qrcode';
import { PrismaService } from '../prisma/prisma.service';
import { FactoryGatewaysService } from '../factory-gateways/factory-gateways.service';
import { CreatePrintAgentDto } from './dto/create-print-agent.dto';
import { CreatePrintJobDto } from './dto/create-print-job.dto';
import { CompletePrintJobDto } from './dto/complete-print-job.dto';

@Injectable()
export class LabelPrintService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly factoryGateways: FactoryGatewaysService,
  ) {}

  async createAgent(dto: CreatePrintAgentDto) {
    const token = `wkpa_${randomBytes(24).toString('base64url')}`;
    const agent = await this.prisma.labelPrintAgent.create({
      data: {
        name: dto.name.trim(),
        printerName: dto.printerName?.trim() || null,
        tokenHash: this.hash(token),
      },
    });
    return { ...this.publicAgent(agent), token };
  }

  async listAgents() {
    const agents = await this.prisma.labelPrintAgent.findMany({
      where: { status: { not: 'DISABLED' } },
      orderBy: { createdAt: 'desc' },
    });
    const now = Date.now();
    return agents.map((agent) => ({
      ...this.publicAgent(agent),
      online: Boolean(agent.lastSeenAt && now - agent.lastSeenAt.getTime() < 45_000),
    }));
  }

  async removeAgent(id: string) {
    const agent = await this.prisma.labelPrintAgent.findUnique({ where: { id } });
    if (!agent) throw new NotFoundException('打印终端不存在');
    await this.prisma.labelPrintAgent.update({
      where: { id },
      data: {
        status: 'DISABLED',
        tokenHash: this.hash(`disabled:${id}:${randomBytes(24).toString('hex')}`),
        lastError: null,
      },
    });
    return { ok: true };
  }

  async createJob(dto: CreatePrintJobDto, username: string) {
    const [agent, gateway] = await Promise.all([
      this.prisma.labelPrintAgent.findUnique({ where: { id: dto.agentId } }),
      this.prisma.factoryGateway.findUnique({ where: { id: dto.factoryGatewayId } }),
    ]);
    if (!agent || agent.status === 'DISABLED') throw new NotFoundException('打印终端不存在');
    if (!gateway || gateway.status === 'REMOVED') throw new NotFoundException('出厂网关不存在');
    if (gateway.status === 'BOUND' || gateway.deviceId) throw new BadRequestException('已绑定网关不能再次打印绑定标签');
    if (!gateway.bindCodeEncrypted) throw new BadRequestException('该网关使用旧版凭证，请先在管理平台重置绑定凭证');
    return this.prisma.labelPrintJob.create({
      data: {
        agentId: dto.agentId,
        factoryGatewayId: dto.factoryGatewayId,
        sn: gateway.sn,
        labelSize: dto.labelSize,
        copies: dto.copies,
        createdBy: username,
      },
      include: { agent: { select: { name: true } } },
    });
  }

  listJobs(page = 1, pageSize = 50) {
    const take = Math.min(100, Math.max(1, pageSize));
    const skip = (Math.max(1, page) - 1) * take;
    return this.prisma.$transaction([
      this.prisma.labelPrintJob.findMany({
        skip, take, orderBy: { createdAt: 'desc' },
        include: { agent: { select: { name: true, printerName: true } } },
      }),
      this.prisma.labelPrintJob.count(),
    ]).then(([items, total]) => ({ items, total }));
  }

  async retryJob(id: string) {
    const job = await this.prisma.labelPrintJob.findUnique({ where: { id } });
    if (!job) throw new NotFoundException('打印任务不存在');
    return this.prisma.labelPrintJob.update({
      where: { id },
      data: { status: 'PENDING', claimedAt: null, completedAt: null, error: null },
    });
  }

  async cancelJob(id: string) {
    const job = await this.prisma.labelPrintJob.findUnique({ where: { id } });
    if (!job) throw new NotFoundException('打印任务不存在');
    if (!['PENDING', 'FAILED', 'PRINTING'].includes(job.status)) throw new BadRequestException('当前任务不能取消');
    return this.prisma.labelPrintJob.update({ where: { id }, data: { status: 'CANCELLED', completedAt: new Date() } });
  }

  async claim(token: string) {
    const agent = await this.authenticateAgent(token);
    const staleAt = new Date(Date.now() - 2 * 60_000);
    await this.prisma.$transaction([
      this.prisma.labelPrintAgent.update({ where: { id: agent.id }, data: { status: 'ONLINE', lastSeenAt: new Date() } }),
      this.prisma.labelPrintJob.updateMany({
        where: { agentId: agent.id, status: 'PRINTING', claimedAt: { lt: staleAt } },
        data: { status: 'PENDING', claimedAt: null, error: '打印终端超时，任务已自动重新排队' },
      }),
    ]);

    const candidate = await this.prisma.labelPrintJob.findFirst({
      where: { agentId: agent.id, status: 'PENDING' },
      orderBy: { createdAt: 'asc' },
    });
    if (!candidate) return { job: null };
    const claimed = await this.prisma.labelPrintJob.updateMany({
      where: { id: candidate.id, status: 'PENDING' },
      data: { status: 'PRINTING', claimedAt: new Date(), attempts: { increment: 1 }, error: null },
    });
    if (!claimed.count) return { job: null };
    const credential = await this.factoryGateways.getBindCredential(candidate.factoryGatewayId);
    const payload = JSON.stringify({
      type: 'weikong-gateway-bind', version: 1, sn: credential.sn.trim().toUpperCase(), bindCode: credential.bindCode.trim(),
    });
    const qrDataUrl = await QRCode.toDataURL(payload, {
      errorCorrectionLevel: 'M', margin: 2, width: 480,
      color: { dark: '#0f172a', light: '#ffffff' },
    });
    return {
      job: {
        id: candidate.id,
        sn: credential.sn,
        bindCode: credential.bindCode,
        qrDataUrl,
        labelSize: candidate.labelSize,
        copies: candidate.copies,
        printerName: agent.printerName,
      },
    };
  }

  async complete(token: string, id: string, dto: CompletePrintJobDto) {
    const agent = await this.authenticateAgent(token);
    const job = await this.prisma.labelPrintJob.findFirst({ where: { id, agentId: agent.id } });
    if (!job) throw new NotFoundException('打印任务不存在');
    await this.prisma.$transaction([
      this.prisma.labelPrintJob.update({
        where: { id },
        data: {
          status: dto.success ? 'SUCCESS' : 'FAILED',
          completedAt: new Date(),
          error: dto.success ? null : (dto.error || '打印失败'),
        },
      }),
      this.prisma.labelPrintAgent.update({
        where: { id: agent.id },
        data: { status: 'ONLINE', lastSeenAt: new Date(), printerName: dto.printerName || agent.printerName, lastError: dto.success ? null : dto.error },
      }),
    ]);
    return { ok: true };
  }

  private async authenticateAgent(token: string) {
    if (!token) throw new UnauthorizedException('缺少打印终端令牌');
    const agent = await this.prisma.labelPrintAgent.findUnique({ where: { tokenHash: this.hash(token) } });
    if (!agent || agent.status === 'DISABLED') throw new UnauthorizedException('打印终端令牌无效');
    return agent;
  }

  private hash(value: string) {
    return createHash('sha256').update(value).digest('hex');
  }

  private publicAgent<T extends { tokenHash: string }>(agent: T) {
    const { tokenHash: _tokenHash, ...publicValue } = agent;
    return publicValue;
  }
}

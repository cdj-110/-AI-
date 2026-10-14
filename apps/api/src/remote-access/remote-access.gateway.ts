import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { IncomingHttpHeaders, IncomingMessage, Server as HttpServer } from 'http';
import { Duplex } from 'stream';
import { randomBytes } from 'crypto';
import * as bcrypt from 'bcryptjs';
import { RawData, WebSocket, WebSocketServer } from 'ws';
import { PrismaService } from '../prisma/prisma.service';

type TunnelRequest = {
  type: 'http.request';
  requestId: string;
  method: string;
  path: string;
  headers: Record<string, string[]>;
  bodyBase64: string;
};

type TunnelResponse = {
  type?: string;
  requestId?: string;
  streamId?: string;
  status?: number;
  headers?: Record<string, string[]>;
  bodyBase64?: string;
  dataBase64?: string;
  binary?: boolean;
  error?: string;
};

type PendingRequest = {
  resolve: (value: TunnelResponse) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};

type AgentConnection = {
  deviceId: string;
  deviceKey: string;
  socket: WebSocket;
  connectedAt: Date;
  lastSeenAt: Date;
};

type BrowserConnection = {
  deviceId: string;
  socket: WebSocket;
  onActivity?: () => void;
};

type TerminalConnection = BrowserConnection & {
  authSubmitted: boolean;
  openTimer?: ReturnType<typeof setTimeout>;
};

@Injectable()
export class RemoteAccessGateway implements OnModuleDestroy {
  private server?: WebSocketServer;
  private httpServer?: HttpServer;
  private upgradeHandler?: (request: IncomingMessage, socket: Duplex, head: Buffer) => void;
  private heartbeatTimer?: ReturnType<typeof setInterval>;
  private readonly agents = new Map<string, AgentConnection>();
  private readonly pending = new Map<string, PendingRequest>();
  private readonly browserSockets = new Map<string, BrowserConnection>();
  private readonly terminalSockets = new Map<string, TerminalConnection>();

  constructor(private readonly prisma: PrismaService) {}

  attach(httpServer: HttpServer) {
    if (this.server) return;
    this.httpServer = httpServer;
    this.server = new WebSocketServer({ noServer: true, clientTracking: false, maxPayload: 32 * 1024 * 1024 });
    this.upgradeHandler = (request, socket, head) => {
      const url = new URL(request.url ?? '/', 'http://localhost');
      if (url.pathname !== '/api/remote-access/tunnel') return;
      this.server?.handleUpgrade(request, socket, head, (client) => void this.accept(client, request, url));
    };
    httpServer.on('upgrade', this.upgradeHandler);
    this.heartbeatTimer = setInterval(() => this.heartbeat(), 30_000);
  }

  status(deviceId: string) {
    const agent = this.agents.get(deviceId);
    return agent && agent.socket.readyState === WebSocket.OPEN
      ? { connected: true, connectedAt: agent.connectedAt, lastSeenAt: agent.lastSeenAt }
      : { connected: false };
  }

  disconnectDevice(deviceId: string) {
    const agent = this.agents.get(deviceId);
    if (agent) agent.socket.close(4003, 'cloud access disabled');
    for (const connection of this.browserSockets.values()) {
      if (connection.deviceId === deviceId) connection.socket.close(4003, 'cloud access disabled');
    }
    for (const connection of this.terminalSockets.values()) {
      if (connection.deviceId === deviceId) connection.socket.close(4003, 'cloud access disabled');
    }
  }

  async request(deviceId: string, input: Omit<TunnelRequest, 'type' | 'requestId'>, timeoutMs = 20_000) {
    const agent = this.onlineAgent(deviceId);
    const requestId = randomBytes(16).toString('hex');
    return new Promise<TunnelResponse>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(requestId);
        reject(new Error('网关响应超时'));
      }, timeoutMs);
      this.pending.set(requestId, { resolve, reject, timer });
      agent.socket.send(JSON.stringify({ type: 'http.request', requestId, ...input } satisfies TunnelRequest), (error) => {
        if (!error) return;
        const pending = this.pending.get(requestId);
        if (!pending) return;
        clearTimeout(pending.timer);
        this.pending.delete(requestId);
        pending.reject(error);
      });
    });
  }

  async openBrowserSocket(deviceId: string, browser: WebSocket, path: string, headers: IncomingHttpHeaders, onActivity?: () => void) {
    const agent = this.onlineAgent(deviceId);
    const streamId = randomBytes(16).toString('hex');
    this.browserSockets.set(streamId, { deviceId, socket: browser, onActivity });
    agent.socket.send(JSON.stringify({
      type: 'ws.open', streamId, path,
      headers: this.forwardHeaders(headers),
    }));
    browser.on('message', (data, isBinary) => {
      onActivity?.();
      this.sendAgent(deviceId, {
        type: 'ws.data', streamId, dataBase64: Buffer.from(data as Buffer).toString('base64'), binary: isBinary,
      });
    });
    const close = () => {
      if (!this.browserSockets.delete(streamId)) return;
      this.sendAgent(deviceId, { type: 'ws.close', streamId });
    };
    browser.on('close', close);
    browser.on('error', close);
  }

  async openTerminalSocket(deviceId: string, browser: WebSocket, onActivity?: () => void) {
    this.onlineAgent(deviceId);
    const streamId = randomBytes(16).toString('hex');
    const terminal: TerminalConnection = { deviceId, socket: browser, onActivity, authSubmitted: false };
    this.terminalSockets.set(streamId, terminal);
    browser.send(JSON.stringify({ type: 'terminal.ready' }));
    browser.on('message', (raw) => {
      onActivity?.();
      if (raw.toString().length > 512 * 1024) {
        browser.close(1009, 'terminal message too large');
        return;
      }
      let input: Record<string, unknown>;
      try { input = JSON.parse(raw.toString()) as Record<string, unknown>; } catch { return; }
      const type = String(input.type ?? '');
      if (type === 'ssh.auth') {
        if (terminal.authSubmitted) return;
        const username = String(input.username ?? '').trim();
        const password = String(input.password ?? '');
        if (!username || username.length > 128 || !password || password.length > 1024) {
          browser.send(JSON.stringify({ type: 'ssh.error', error: '请输入有效的 SSH 账号和密码' }));
          return;
        }
        terminal.authSubmitted = true;
        terminal.openTimer = setTimeout(() => {
          if (this.terminalSockets.get(streamId) !== terminal) return;
          browser.send(JSON.stringify({ type: 'ssh.error', error: 'SSH 连接超时，请确认网关已开启 SSH 服务' }), () => {
            browser.close(1013, 'ssh connection timeout');
          });
        }, 20_000);
        this.sendAgent(deviceId, {
          type: 'ssh.open', streamId, username, password,
          cols: this.terminalDimension(input.cols, 120, 20, 500),
          rows: this.terminalDimension(input.rows, 32, 5, 200),
        });
        return;
      }
      if (!terminal.authSubmitted) return;
      if (type === 'ssh.input') {
        const dataBase64 = String(input.dataBase64 ?? '');
        if (dataBase64.length <= 384 * 1024) this.sendAgent(deviceId, { type, streamId, dataBase64 });
      } else if (type === 'ssh.resize') {
        this.sendAgent(deviceId, {
          type, streamId,
          cols: this.terminalDimension(input.cols, 120, 20, 500),
          rows: this.terminalDimension(input.rows, 32, 5, 200),
        });
      }
    });
    const close = () => {
      if (!this.terminalSockets.delete(streamId)) return;
      if (terminal.openTimer) clearTimeout(terminal.openTimer);
      this.sendAgent(deviceId, { type: 'ssh.close', streamId });
    };
    browser.on('close', close);
    browser.on('error', close);
  }

  private async accept(socket: WebSocket, request: IncomingMessage, url: URL) {
    // Install an error listener before the asynchronous database lookup so a
    // client disconnect during authentication cannot become an uncaught event.
    socket.on('error', () => undefined);
    try {
      const deviceKey = (url.searchParams.get('gatewayKey') ?? '').trim();
      const username = String(request.headers['x-gateway-username'] ?? '');
      const password = String(request.headers['x-gateway-password'] ?? '');
      if (!deviceKey || !username || !password) throw new Error('missing credentials');
      const device = await this.authenticate(deviceKey, username, password);
      if (!device) throw new Error('invalid credentials');
      const previous = this.agents.get(device.id);
      if (previous && previous.socket !== socket) previous.socket.close(4002, 'replaced by a newer tunnel');
      const connection: AgentConnection = { deviceId: device.id, deviceKey, socket, connectedAt: new Date(), lastSeenAt: new Date() };
      this.agents.set(device.id, connection);
      socket.on('pong', () => { connection.lastSeenAt = new Date(); });
      socket.on('message', (data) => this.handleAgentMessage(connection, data));
      socket.on('close', () => this.release(connection));
      socket.on('error', () => this.release(connection));
      socket.send(JSON.stringify({ type: 'auth.ok', protocol: 1, deviceId: device.id }));
    } catch {
      socket.close(4001, 'authentication failed');
    }
  }

  private async authenticate(deviceKey: string, username: string, password: string) {
    const device = await this.prisma.device.findFirst({
      where: { deviceKey, gatewayId: null, tenant: { deviceCloudAccessDisabled: false } },
      select: { id: true, deviceKey: true, deviceType: true, cloudAccessEnabled: true, mqttUsername: true, mqttPasswordHash: true, factoryGateway: { select: { hardwareId: true, deviceSecretHash: true, status: true } } },
    });
    if (!device || !device.cloudAccessEnabled || device.deviceType !== 'GATEWAY' || !password) return null;
    if (username === device.mqttUsername && await bcrypt.compare(password, device.mqttPasswordHash)) return device;
    const factory = device.factoryGateway;
    if (factory && factory.status === 'BOUND' && username === `factory:${factory.hardwareId}` && await bcrypt.compare(password, factory.deviceSecretHash)) return device;
    return null;
  }

  private handleAgentMessage(agent: AgentConnection, raw: RawData) {
    agent.lastSeenAt = new Date();
    let message: TunnelResponse;
    try { message = JSON.parse(raw.toString()) as TunnelResponse; } catch { return; }
    if (message.type === 'http.response' && message.requestId) {
      const pending = this.pending.get(message.requestId);
      if (!pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(message.requestId);
      pending.resolve(message);
      return;
    }
    if (message.type === 'ws.data' && message.streamId) {
      const browser = this.browserSockets.get(message.streamId);
      browser?.onActivity?.();
      if (browser?.socket.readyState === WebSocket.OPEN) {
        const data = Buffer.from(message.dataBase64 ?? '', 'base64');
        browser.socket.send(message.binary ? data : data.toString('utf8'));
      }
      return;
    }
    if ((message.type === 'ws.close' || message.type === 'ws.error') && message.streamId) {
      const browser = this.browserSockets.get(message.streamId);
      this.browserSockets.delete(message.streamId);
      browser?.socket.close(message.type === 'ws.error' ? 1011 : 1000, (message.error ?? '').slice(0, 100));
      return;
    }
    if (message.streamId && ['ssh.opened', 'ssh.data'].includes(message.type ?? '')) {
      const terminal = this.terminalSockets.get(message.streamId);
      if (message.type === 'ssh.opened' && terminal?.openTimer) {
        clearTimeout(terminal.openTimer);
        terminal.openTimer = undefined;
      }
      terminal?.onActivity?.();
      if (terminal?.socket.readyState === WebSocket.OPEN) {
        terminal.socket.send(JSON.stringify({
          type: message.type,
          ...(message.dataBase64 ? { dataBase64: message.dataBase64 } : {}),
        }));
      }
      return;
    }
    if (message.streamId && (message.type === 'ssh.close' || message.type === 'ssh.error')) {
      const terminal = this.terminalSockets.get(message.streamId);
      this.terminalSockets.delete(message.streamId);
      if (terminal?.openTimer) clearTimeout(terminal.openTimer);
      if (terminal?.socket.readyState === WebSocket.OPEN) {
        terminal.socket.send(JSON.stringify({ type: message.type, error: message.error }), () => {
          terminal.socket.close(message.type === 'ssh.error' ? 1011 : 1000);
        });
      }
    }
  }

  private terminalDimension(value: unknown, fallback: number, minimum: number, maximum: number) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? Math.min(maximum, Math.max(minimum, Math.floor(parsed))) : fallback;
  }

  private onlineAgent(deviceId: string) {
    const agent = this.agents.get(deviceId);
    if (!agent || agent.socket.readyState !== WebSocket.OPEN) throw new Error('网关远程通道未连接');
    return agent;
  }

  private sendAgent(deviceId: string, message: Record<string, unknown>) {
    try { this.onlineAgent(deviceId).socket.send(JSON.stringify(message)); } catch { /* browser close path */ }
  }

  private forwardHeaders(headers: IncomingHttpHeaders) {
    const result: Record<string, string[]> = {};
    for (const key of ['cookie', 'sec-websocket-protocol']) {
      const value = headers[key];
      if (value) result[key] = Array.isArray(value) ? value : [value];
    }
    return result;
  }

  private heartbeat() {
    for (const agent of this.agents.values()) {
      if (Date.now() - agent.lastSeenAt.getTime() > 75_000) agent.socket.terminate();
      else agent.socket.ping();
    }
  }

  private release(agent: AgentConnection) {
    if (this.agents.get(agent.deviceId)?.socket !== agent.socket) return;
    this.agents.delete(agent.deviceId);
    for (const [streamId, browser] of this.browserSockets) {
      if (browser.deviceId === agent.deviceId) {
        this.browserSockets.delete(streamId);
        browser.socket.close(1013, 'gateway tunnel disconnected');
      }
    }
    for (const [streamId, terminal] of this.terminalSockets) {
      if (terminal.deviceId === agent.deviceId) {
        this.terminalSockets.delete(streamId);
        if (terminal.openTimer) clearTimeout(terminal.openTimer);
        terminal.socket.close(1013, 'gateway tunnel disconnected');
      }
    }
  }

  onModuleDestroy() {
    if (this.heartbeatTimer) clearInterval(this.heartbeatTimer);
    if (this.httpServer && this.upgradeHandler) this.httpServer.off('upgrade', this.upgradeHandler);
    for (const pending of this.pending.values()) { clearTimeout(pending.timer); pending.reject(new Error('远程访问服务正在停止')); }
    for (const agent of this.agents.values()) agent.socket.close(1001, 'server shutdown');
    for (const browser of this.browserSockets.values()) browser.socket.close(1001, 'server shutdown');
    for (const terminal of this.terminalSockets.values()) {
      if (terminal.openTimer) clearTimeout(terminal.openTimer);
      terminal.socket.close(1001, 'server shutdown');
    }
    this.pending.clear(); this.agents.clear(); this.browserSockets.clear(); this.terminalSockets.clear(); this.server?.close();
  }
}

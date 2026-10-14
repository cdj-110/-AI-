import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { IncomingMessage, Server as HttpServer } from 'http';
import { Duplex } from 'stream';
import { Subscription } from 'rxjs';
import { RawData, WebSocket, WebSocketServer } from 'ws';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { JwtStrategy } from '../auth/jwt.strategy';
import { DevicesService } from './devices.service';

type ClientState = {
  actor?: AuthUser;
  subscription?: Subscription;
  authTimer?: ReturnType<typeof setTimeout>;
  alive: boolean;
  generation: number;
};

type ClientMessage = {
  type?: string;
  token?: string;
  deviceId?: string;
};

@Injectable()
export class DevicesRealtimeGateway implements OnModuleDestroy {
  private server?: WebSocketServer;
  private httpServer?: HttpServer;
  private upgradeHandler?: (request: IncomingMessage, socket: Duplex, head: Buffer) => void;
  private heartbeatTimer?: ReturnType<typeof setInterval>;
  private readonly clients = new Map<WebSocket, ClientState>();

  constructor(
    private readonly jwtService: JwtService,
    private readonly jwtStrategy: JwtStrategy,
    private readonly devicesService: DevicesService,
  ) {}

  attach(httpServer: HttpServer) {
    if (this.server) return;
    this.httpServer = httpServer;
    this.server = new WebSocketServer({ noServer: true, clientTracking: false });
    this.upgradeHandler = (request, socket, head) => {
      const pathname = new URL(request.url ?? '/', 'http://localhost').pathname;
      if (pathname !== '/api/realtime') return;
      this.server?.handleUpgrade(request, socket, head, (client) => this.accept(client));
    };
    httpServer.on('upgrade', this.upgradeHandler);
    this.heartbeatTimer = setInterval(() => void this.heartbeat(), 30_000);
  }

  private accept(client: WebSocket) {
    const state: ClientState = { alive: true, generation: 0 };
    state.authTimer = setTimeout(() => {
      this.send(client, { type: 'auth.error', message: '认证超时' });
      client.close(4001, 'authentication timeout');
    }, 10_000);
    this.clients.set(client, state);
    client.on('pong', () => {
      const current = this.clients.get(client);
      if (current) current.alive = true;
    });
    client.on('message', (data) => void this.handleMessage(client, data));
    client.on('close', () => this.release(client));
    client.on('error', () => this.release(client));
    this.send(client, { type: 'connected', protocol: 1, authRequired: true });
  }

  private async handleMessage(client: WebSocket, raw: RawData) {
    const state = this.clients.get(client);
    if (!state) return;
    let message: ClientMessage;
    try {
      message = JSON.parse(raw.toString()) as ClientMessage;
    } catch {
      this.send(client, { type: 'request.error', message: '消息必须是 JSON' });
      return;
    }

    if (message.type === 'auth') {
      await this.authenticate(client, state, message.token);
      return;
    }
    if (!state.actor) {
      this.send(client, { type: 'auth.error', message: '请先完成认证' });
      return;
    }
    if (message.type === 'telemetry.subscribe') {
      await this.subscribeTelemetry(client, state, message.deviceId);
      return;
    }
    if (message.type === 'ping') {
      this.send(client, { type: 'pong', sentAt: new Date().toISOString() });
      return;
    }
    this.send(client, { type: 'request.error', message: '不支持的消息类型' });
  }

  private async authenticate(client: WebSocket, state: ClientState, token?: string) {
    if (!token) {
      this.send(client, { type: 'auth.error', message: '缺少访问令牌' });
      client.close(4001, 'missing token');
      return;
    }
    try {
      const payload = await this.jwtService.verifyAsync<AuthUser>(token);
      if (!payload?.sub || !payload.role) throw new Error('invalid token payload');
      state.actor = await this.jwtStrategy.validate(payload);
      if (state.authTimer) clearTimeout(state.authTimer);
      state.authTimer = undefined;
      this.send(client, { type: 'auth.ok' });
    } catch {
      this.send(client, { type: 'auth.error', message: '登录已失效，请重新登录' });
      client.close(4001, 'invalid token');
    }
  }

  private async subscribeTelemetry(client: WebSocket, state: ClientState, deviceId?: string) {
    if (!deviceId) {
      this.send(client, { type: 'subscription.error', message: '缺少设备编号' });
      return;
    }
    const generation = ++state.generation;
    state.subscription?.unsubscribe();
    state.subscription = undefined;
    try {
      const stream = await this.devicesService.telemetryStream(state.actor!, deviceId);
      if (!this.clients.has(client) || generation !== state.generation) return;
      state.subscription = stream.subscribe({
        next: (event) => this.send(client, {
          type: 'telemetry.update',
          deviceId,
          payload: event.data,
          sentAt: new Date().toISOString(),
        }),
        error: (error) => this.send(client, { type: 'subscription.error', deviceId, message: this.errorMessage(error) }),
      });
      this.send(client, { type: 'telemetry.subscribed', deviceId });
    } catch (error) {
      this.send(client, { type: 'subscription.error', deviceId, message: this.errorMessage(error) });
    }
  }

  private send(client: WebSocket, message: Record<string, unknown>) {
    if (client.readyState !== WebSocket.OPEN) return;
    client.send(JSON.stringify(message));
  }

  private async heartbeat() {
    for (const [client, state] of this.clients) {
      if (!state.alive) {
        client.terminate();
        this.release(client);
        continue;
      }
      if (state.actor) {
        try {
          state.actor = await this.jwtStrategy.validate(state.actor);
        } catch {
          this.send(client, { type: 'auth.error', message: '登录已失效，请重新登录' });
          client.close(4001, 'session expired');
          this.release(client);
          continue;
        }
      }
      state.alive = false;
      client.ping();
    }
  }

  private release(client: WebSocket) {
    const state = this.clients.get(client);
    if (!state) return;
    if (state.authTimer) clearTimeout(state.authTimer);
    state.subscription?.unsubscribe();
    this.clients.delete(client);
  }

  private errorMessage(error: unknown) {
    return error instanceof Error ? error.message : '订阅失败';
  }

  onModuleDestroy() {
    if (this.heartbeatTimer) clearInterval(this.heartbeatTimer);
    if (this.httpServer && this.upgradeHandler) this.httpServer.off('upgrade', this.upgradeHandler);
    for (const client of this.clients.keys()) client.close(1001, 'server shutdown');
    this.clients.clear();
    this.server?.close();
    this.server = undefined;
  }
}

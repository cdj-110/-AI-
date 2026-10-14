import { BadRequestException, ForbiddenException, Injectable, NotFoundException, OnModuleDestroy, ServiceUnavailableException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { IncomingMessage, Server as HttpServer, createServer } from 'http';
import { randomBytes } from 'crypto';
import { Duplex } from 'stream';
import Redis from 'ioredis';
import { WebSocketServer } from 'ws';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { RemoteAccessGateway } from './remote-access.gateway';
import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';
import { deviceAccessWhere } from '../common/user-permissions';

type SessionMode = 'web' | 'ssh';
type Session = { id: string; token: string; deviceId: string; userId: string; tenantId: string; mode: SessionMode; createdAt: string; expiresAt: string };

const remoteSessionCookie = 'wk_remote_session_v2';
const legacyRemoteSessionCookie = 'wk_remote_session';

@Injectable()
export class RemoteAccessService implements OnModuleDestroy {
  private readonly redis: Redis;
  private readonly sessionPrefix: string;
  private readonly sessionIdleSeconds: number;
  private readonly linkTtlSeconds: number;
  private readonly cookieMaxAgeSeconds: number;
  private readonly lastSessionTouch = new Map<string, number>();
  private readonly browserSessionTimers = new Set<ReturnType<typeof setInterval>>();
  private proxyServer?: HttpServer;

  constructor(
    private readonly prisma: PrismaService,
    private readonly gateway: RemoteAccessGateway,
    private readonly config: ConfigService,
  ) {
    this.sessionPrefix = this.config.get<string>('REMOTE_ACCESS_SESSION_PREFIX', 'weikong:remote-access');
    this.sessionIdleSeconds = this.boundedSeconds('REMOTE_ACCESS_SESSION_IDLE_SECONDS', 15 * 60, 60, 24 * 60 * 60);
    this.linkTtlSeconds = this.boundedSeconds('REMOTE_ACCESS_LINK_TTL_SECONDS', 15 * 60, 30, this.sessionIdleSeconds);
    // Cookie 只保存会话标识，真正的空闲有效期由 Redis 控制。较长的 Cookie 生命周期可避免
    // 仅通过 WebSocket 持续操作时，浏览器 Cookie 先于已续期的 Redis 会话失效。
    this.cookieMaxAgeSeconds = this.boundedSeconds(
      'REMOTE_ACCESS_COOKIE_MAX_AGE_SECONDS',
      7 * 24 * 60 * 60,
      this.sessionIdleSeconds,
      30 * 24 * 60 * 60,
    );
    this.redis = new Redis({
      host: this.config.get<string>('REDIS_HOST', '127.0.0.1'),
      port: this.config.get<number>('REDIS_PORT', 6379),
      password: this.config.get<string>('REDIS_PASSWORD') || undefined,
      db: this.config.get<number>('REDIS_DB', 0),
      maxRetriesPerRequest: 2,
      enableReadyCheck: true,
      lazyConnect: false,
    });
    // Redis 会自动重连；错误由调用路径转成明确的服务不可用提示，避免未监听 error 事件终止进程。
    this.redis.on('error', () => undefined);
  }

  startProxy() {
    if (this.proxyServer) return;
    const port = this.config.get<number>('REMOTE_ACCESS_PORT', 3201);
    const server = createServer((request, response) => void this.proxy(request, response));
    const browserWss = new WebSocketServer({ noServer: true, clientTracking: false, maxPayload: 8 * 1024 * 1024 });
    server.on('upgrade', (request: IncomingMessage, socket: Duplex, head: Buffer) => {
      void this.upgradeBrowserSocket(browserWss, request, socket, head);
    });
    server.listen(port, '0.0.0.0');
    this.proxyServer = server;
  }

  async status(actor: AuthUser, deviceId: string) {
    await this.accessibleGateway(actor, deviceId);
    return this.gateway.status(deviceId);
  }

  async createSession(actor: AuthUser, deviceId: string, requestOrigin: string) {
    const session = await this.createStoredSession(actor, deviceId, 'web');
    const publicBase = this.publicBase(requestOrigin);
    return { url: `${publicBase}/connect/${session.token}`, expiresAt: new Date(session.expiresAt), sessionId: session.id };
  }

  async createTerminalSession(actor: AuthUser, deviceId: string, requestOrigin: string) {
    const session = await this.createStoredSession(actor, deviceId, 'ssh');
    const publicURL = new URL(this.publicBase(requestOrigin));
    publicURL.protocol = publicURL.protocol === 'https:' ? 'wss:' : 'ws:';
    publicURL.pathname = `/terminal/connect/${session.token}`;
    publicURL.search = '';
    publicURL.hash = '';
    return { url: publicURL.toString(), expiresAt: new Date(session.expiresAt), sessionId: session.id };
  }

  private async createStoredSession(actor: AuthUser, deviceId: string, mode: SessionMode) {
    await this.accessibleGateway(actor, deviceId);
    if (!this.gateway.status(deviceId).connected) throw new BadRequestException('网关远程通道未连接，请先在网关启用远程访问');
    const now = new Date();
    const expiresAt = new Date(now.getTime() + this.linkTtlSeconds * 1000);
    const session: Session = {
      id: randomBytes(24).toString('base64url'), token: randomBytes(32).toString('base64url'),
      deviceId, userId: actor.sub, tenantId: actor.tenantId ?? '', createdAt: now.toISOString(),
      expiresAt: expiresAt.toISOString(), mode,
    };
    try {
      await this.redis.multi()
        .set(this.tokenKey(session.token), JSON.stringify(session), 'EX', this.linkTtlSeconds)
        .set(this.sessionKey(session.id), JSON.stringify(session), 'EX', this.sessionIdleSeconds)
        .exec();
    } catch {
      throw new ServiceUnavailableException('远程访问会话服务暂不可用，请稍后重试');
    }
    return session;
  }

  private async accessibleGateway(actor: AuthUser, deviceId: string) {
    const device = await this.prisma.device.findFirst({ where: { AND: [{ id: deviceId }, deviceAccessWhere(actor)] }, select: { id: true, tenantId: true, deviceType: true, status: true, cloudAccessEnabled: true, tenant: { select: cloudPolicyTenantSelect } } });
    if (tenantBlocksCloud(device)) throw new ForbiddenException('所属租户已停用设备云连接');
    if (!device) throw new NotFoundException('网关不存在');
    if (device.deviceType !== 'GATEWAY') throw new BadRequestException('只有网关设备支持远程访问');
    if (!device.cloudAccessEnabled) throw new ForbiddenException('该网关的云连接已被管理平台禁用');
    return device;
  }

  private async proxy(request: IncomingMessage, response: import('http').ServerResponse) {
    this.securityHeaders(response);
    const url = new URL(request.url ?? '/', 'http://remote.local');
    if (url.pathname.startsWith('/connect/')) {
      const token = url.pathname.slice('/connect/'.length);
      let session: Session | undefined;
      try {
        session = await this.consumeConnectionToken(token);
      } catch {
        response.writeHead(503, { 'Content-Type': 'text/plain; charset=utf-8' });
        response.end('远程访问会话服务暂不可用，请稍后重试。');
        return;
      }
      if (!session) { response.writeHead(401); response.end('远程访问链接已失效'); return; }
      if (session.mode !== 'web') { response.writeHead(400); response.end('会话类型不匹配'); return; }
      this.renewSessionCookie(response, session.id);
      response.writeHead(302, { Location: '/' }); response.end(); return;
    }
    if (url.pathname === '/remote-healthz') { response.writeHead(200, { 'Content-Type': 'application/json' }); response.end('{"status":"ok"}'); return; }
    let session: Session | undefined;
    try {
      session = await this.sessionFromCookie(request.headers.cookie);
    } catch {
      response.writeHead(503, { 'Content-Type': 'text/plain; charset=utf-8' });
      response.end('远程访问会话服务暂不可用，请稍后重试。');
      return;
    }
    if (!session) {
      response.writeHead(401, { 'Content-Type': 'text/html; charset=utf-8' });
      response.end(this.expiredSessionPage());
      return;
    }
    try {
      const body = await this.readBody(request);
      const result = await this.gateway.request(session.deviceId, {
        method: request.method ?? 'GET', path: `${url.pathname}${url.search}`,
        headers: this.proxyHeaders(request.headers), bodyBase64: body.toString('base64'),
      });
      if (result.error) throw new Error(result.error);
      for (const [key, values] of Object.entries(result.headers ?? {})) {
        if (!this.isHopByHop(key)) response.setHeader(key, values);
      }
      // 每次有效 HTTP 活动同时刷新浏览器 Cookie；若网关页面也设置 Cookie，则保留双方 Cookie。
      this.renewSessionCookie(response, session.id);
      response.writeHead(result.status ?? 502); response.end(Buffer.from(result.bodyBase64 ?? '', 'base64'));
    } catch (error) {
      response.writeHead(502, { 'Content-Type': 'text/plain; charset=utf-8' });
      response.end(error instanceof Error ? error.message : '远程网关访问失败');
    }
  }

  private async upgradeBrowserSocket(browserWss: WebSocketServer, request: IncomingMessage, socket: Duplex, head: Buffer) {
    const url = new URL(request.url ?? '/', 'http://remote.local');
    if (url.pathname.startsWith('/terminal/connect/')) {
      await this.upgradeTerminalSocket(browserWss, request, socket, head, url.pathname.slice('/terminal/connect/'.length));
      return;
    }
    let session: Session | undefined;
    try {
      session = await this.sessionFromCookie(request.headers.cookie);
    } catch {
      socket.write('HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }
    if (!session) {
      socket.write('HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }
    if (session.mode !== 'web') {
      socket.write('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }
    browserWss.handleUpgrade(request, socket, head, (browser) => {
      // A remote page can stay open without producing application messages for a long time.
      // Keep the Redis idle lease alive while its browser WebSocket is still connected;
      // otherwise a quiet but open page expires after the configured idle timeout.
      const renewEveryMs = Math.max(15_000, Math.min(60_000, Math.floor(this.sessionIdleSeconds * 1000 / 3)));
      const sessionTimer = setInterval(() => void this.touchSession(session!.id), renewEveryMs);
      this.browserSessionTimers.add(sessionTimer);
      const stopRenewing = () => {
        clearInterval(sessionTimer);
        this.browserSessionTimers.delete(sessionTimer);
      };
      browser.once('close', stopRenewing);
      browser.once('error', stopRenewing);
      void this.gateway.openBrowserSocket(
        session!.deviceId,
        browser,
        request.url ?? '/',
        request.headers,
        () => void this.touchSession(session!.id),
      ).catch(() => browser.close(1013, 'gateway tunnel unavailable'));
    });
  }

  private async upgradeTerminalSocket(browserWss: WebSocketServer, request: IncomingMessage, socket: Duplex, head: Buffer, token: string) {
    let session: Session | undefined;
    try {
      session = await this.consumeConnectionToken(token);
    } catch {
      socket.write('HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }
    if (!session || session.mode !== 'ssh') {
      socket.write('HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }
    browserWss.handleUpgrade(request, socket, head, (browser) => {
      let idleTimer: ReturnType<typeof setTimeout>;
      const stop = () => {
        clearTimeout(idleTimer);
        this.browserSessionTimers.delete(idleTimer);
      };
      const activity = () => {
        if (idleTimer) {
          clearTimeout(idleTimer);
          this.browserSessionTimers.delete(idleTimer);
        }
        void this.touchSession(session!.id);
        idleTimer = setTimeout(() => browser.close(4001, 'terminal session idle timeout'), this.sessionIdleSeconds * 1000);
        this.browserSessionTimers.add(idleTimer);
      };
      activity();
      browser.once('close', stop);
      browser.once('error', stop);
      void this.gateway.openTerminalSocket(session!.deviceId, browser, activity)
        .catch(() => browser.close(1013, 'gateway tunnel unavailable'));
    });
  }

  private async consumeConnectionToken(token: string) {
    if (!token) return undefined;
    let raw: string | null;
    try {
      raw = await this.redis.getdel(this.tokenKey(token));
    } catch {
      throw new ServiceUnavailableException('远程访问会话服务暂不可用，请稍后重试');
    }
    const session = this.parseSession(raw);
    if (!session) return undefined;
    // 连接令牌是一次性的；只有对应会话仍存在时才允许建立浏览器 Cookie。
    const active = await this.loadSession(session.id, true);
    return active;
  }

  private async sessionFromCookie(cookie = '') {
    const values = cookie.split(';').map((item) => item.trim());
    const ids = [remoteSessionCookie, legacyRemoteSessionCookie]
      .flatMap((name) => values.filter((item) => item.startsWith(`${name}=`)).map((item) => item.slice(name.length + 1)))
      .filter((id, index, all) => Boolean(id) && all.indexOf(id) === index);
    // Browsers can retain cookies with the same name but different legacy
    // domain/path attributes. Try every candidate instead of rejecting the
    // request because the first cookie happens to reference an expired session.
    for (const id of ids) {
      const session = await this.loadSession(id, true);
      if (session) return session;
    }
    return undefined;
  }

  private async loadSession(id: string, renew: boolean) {
    let raw: string | null;
    try {
      raw = renew
        ? await this.redis.getex(this.sessionKey(id), 'EX', this.sessionIdleSeconds)
        : await this.redis.get(this.sessionKey(id));
    } catch {
      throw new ServiceUnavailableException('远程访问会话服务暂不可用，请稍后重试');
    }
    return this.parseSession(raw);
  }

  private async touchSession(id: string) {
    const now = Date.now();
    if (now - (this.lastSessionTouch.get(id) ?? 0) < 30_000) return;
    this.lastSessionTouch.set(id, now);
    try {
      const renewed = await this.redis.expire(this.sessionKey(id), this.sessionIdleSeconds);
      if (!renewed) this.lastSessionTouch.delete(id);
    } catch {
      this.lastSessionTouch.delete(id);
    }
  }

  private parseSession(raw: string | null) {
    if (!raw) return undefined;
    try {
      const value = JSON.parse(raw) as Session;
      if (!value?.id || !value.deviceId || !value.userId) return undefined;
      value.mode = value.mode === 'ssh' ? 'ssh' : 'web';
      return value;
    } catch {
      return undefined;
    }
  }

  private readBody(request: IncomingMessage) {
    return new Promise<Buffer>((resolve, reject) => {
      const chunks: Buffer[] = []; let total = 0;
      request.on('data', (chunk: Buffer) => { total += chunk.length; if (total > 16 * 1024 * 1024) { reject(new Error('请求内容超过 16MB')); request.destroy(); } else chunks.push(chunk); });
      request.on('end', () => resolve(Buffer.concat(chunks))); request.on('error', reject);
    });
  }

  private proxyHeaders(headers: IncomingMessage['headers']) {
    const result: Record<string, string[]> = {};
    for (const [key, value] of Object.entries(headers)) {
      if (!value || this.isHopByHop(key) || key === 'host') continue;
      result[key] = Array.isArray(value) ? value : [value];
    }
    return result;
  }

  private isHopByHop(key: string) { return ['connection', 'upgrade', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding'].includes(key.toLowerCase()); }
  private securityHeaders(response: import('http').ServerResponse) { response.setHeader('X-Content-Type-Options', 'nosniff'); response.setHeader('Referrer-Policy', 'no-referrer'); response.setHeader('Cache-Control', 'no-store'); }
  private defaultPublicURL(origin: string) { const url = new URL(origin); url.port = String(this.config.get<number>('REMOTE_ACCESS_PORT', 3201)); return url.origin; }
  private publicBase(origin: string) { return (this.config.get<string>('REMOTE_ACCESS_PUBLIC_URL') ?? '').replace(/\/$/, '') || this.defaultPublicURL(origin); }
  private sessionKey(id: string) { return `${this.sessionPrefix}:session:${id}`; }
  private tokenKey(token: string) { return `${this.sessionPrefix}:token:${token}`; }
  private renewSessionCookie(response: import('http').ServerResponse, sessionId: string) {
    const secure = (this.config.get<string>('REMOTE_ACCESS_PUBLIC_URL') ?? '').toLowerCase().startsWith('https://') ? '; Secure' : '';
    // Lax permits the cloud console (a different port/origin) to open this
    // top-level session while still blocking third-party subresource requests.
    const expires = new Date(Date.now() + this.cookieMaxAgeSeconds * 1000).toUTCString();
    const cookie = `${remoteSessionCookie}=${sessionId}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${this.cookieMaxAgeSeconds}; Expires=${expires}${secure}`;
    const existing = response.getHeader('Set-Cookie');
    const cookies = existing === undefined ? [] : Array.isArray(existing) ? existing.map(String) : [String(existing)];
    response.setHeader('Set-Cookie', [...cookies, cookie]);
  }
  private expiredSessionPage() {
    return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>远程访问会话已失效</title>
  <style>
    *{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f3f7fc;color:#17233b;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Microsoft YaHei",sans-serif}.card{width:min(460px,calc(100vw - 32px));padding:36px;border:1px solid #dbe7f5;border-radius:18px;background:#fff;box-shadow:0 18px 50px rgba(37,99,235,.1);text-align:center}.icon{display:grid;place-items:center;width:58px;height:58px;margin:0 auto 18px;border-radius:18px;background:#fff3e8;color:#e67700;font-size:30px}.card h1{margin:0;font-size:22px}.card p{margin:12px 0 24px;color:#65758b;font-size:14px;line-height:1.8}.card button{height:40px;padding:0 22px;border:0;border-radius:8px;background:#1677ff;color:#fff;font-size:14px;cursor:pointer}.hint{margin-top:16px!important;color:#94a3b8!important;font-size:12px!important}
  </style>
</head>
<body>
  <main class="card">
    <div class="icon">!</div>
    <h1>远程访问会话已失效</h1>
    <p>为保护网关安全，长时间未使用的远程会话会自动结束。请关闭此页面，返回云平台后重新点击“打开远程访问”。</p>
    <button type="button" onclick="window.close()">关闭此页面</button>
    <p class="hint">已清理旧会话标识，重新打开后将自动建立新会话。</p>
  </main>
</body>
</html>`;
  }
  private boundedSeconds(name: string, fallback: number, minimum: number, maximum: number) {
    const value = Number(this.config.get<string | number>(name, fallback));
    return Number.isFinite(value) ? Math.min(maximum, Math.max(minimum, Math.floor(value))) : fallback;
  }

  async onModuleDestroy() {
    this.proxyServer?.close();
    for (const timer of this.browserSessionTimers) clearInterval(timer);
    this.browserSessionTimers.clear();
    this.lastSessionTouch.clear();
    try { await this.redis.quit(); } catch { this.redis.disconnect(); }
  }
}

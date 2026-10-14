import { HttpException, HttpStatus, Injectable, OnModuleDestroy, ServiceUnavailableException, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { createHash, randomBytes } from 'crypto';
import Redis from 'ioredis';

export type AuthPortal = 'admin' | 'tenant' | 'legacy';

type StoredAuthSession = {
  id: string;
  userId: string;
  portal: AuthPortal;
  createdAt: string;
  absoluteExpiresAt: string;
};

@Injectable()
export class AuthSessionService implements OnModuleDestroy {
  private readonly redis: Redis;
  private readonly prefix: string;
  private readonly adminIdleSeconds: number;
  private readonly tenantIdleSeconds: number;
  private readonly absoluteSeconds: number;

  constructor(private readonly config: ConfigService) {
    this.prefix = this.config.get<string>('AUTH_SESSION_PREFIX', 'weikong:auth-session');
    this.adminIdleSeconds = this.boundedSeconds('AUTH_ADMIN_IDLE_SECONDS', 30 * 60, 5 * 60, 24 * 60 * 60);
    this.tenantIdleSeconds = this.boundedSeconds('AUTH_TENANT_IDLE_SECONDS', 60 * 60, 5 * 60, 24 * 60 * 60);
    this.absoluteSeconds = this.boundedSeconds('AUTH_SESSION_ABSOLUTE_SECONDS', 12 * 60 * 60, 30 * 60, 7 * 24 * 60 * 60);
    this.redis = new Redis({
      host: this.config.get<string>('REDIS_HOST', '127.0.0.1'),
      port: this.config.get<number>('REDIS_PORT', 6379),
      password: this.config.get<string>('REDIS_PASSWORD') || undefined,
      db: this.config.get<number>('REDIS_DB', 0),
      maxRetriesPerRequest: 1,
    });
    this.redis.on('error', () => undefined);
  }

  async create(userId: string, portal: AuthPortal) {
    const now = new Date();
    const session: StoredAuthSession = {
      id: randomBytes(32).toString('base64url'),
      userId,
      portal,
      createdAt: now.toISOString(),
      absoluteExpiresAt: new Date(now.getTime() + this.absoluteSeconds * 1000).toISOString(),
    };
    await this.set(session);
    return { session, idleTimeoutSeconds: this.idleSeconds(portal), absoluteTimeoutSeconds: this.absoluteSeconds };
  }

  async validate(id: string, userId: string, portal?: AuthPortal) {
    const session = await this.read(id);
    if (!session || session.userId !== userId || (portal && session.portal !== portal)) {
      throw new UnauthorizedException('登录已失效，请重新登录');
    }
    if (Date.now() >= new Date(session.absoluteExpiresAt).getTime()) {
      await this.redis.del(this.key(id)).catch(() => undefined);
      throw new UnauthorizedException('登录已达最长使用时间，请重新登录');
    }
    return session;
  }

  async touch(id: string, userId: string, portal?: AuthPortal) {
    const session = await this.validate(id, userId, portal);
    await this.set(session);
    return { idleTimeoutSeconds: this.idleSeconds(session.portal), absoluteExpiresAt: session.absoluteExpiresAt };
  }

  async revoke(id?: string) {
    if (id) await this.redis.del(this.key(id)).catch(() => undefined);
  }

  async assertLoginAllowed(account: string, ip: string) {
    try {
      const [accountFailures, ipFailures] = await this.redis.mget(this.loginFailureKey('account', `${ip}\0${account}`), this.loginFailureKey('ip', ip));
      if (Number(accountFailures || 0) >= 5 || Number(ipFailures || 0) >= 20) {
        throw new HttpException('登录失败次数过多，请 5 分钟后重试', HttpStatus.TOO_MANY_REQUESTS);
      }
    } catch (error) {
      if (error instanceof HttpException && error.getStatus() === HttpStatus.TOO_MANY_REQUESTS) throw error;
      throw new ServiceUnavailableException('登录会话服务暂时不可用');
    }
  }

  async requiresCaptcha(account: string, ip: string) {
    try {
      const [accountFailures, ipFailures] = await this.redis.mget(
        this.loginFailureKey('account', `${ip}\0${account}`),
        this.loginFailureKey('ip', ip),
      );
      return Number(accountFailures || 0) >= 1 || Number(ipFailures || 0) >= 3;
    } catch {
      throw new ServiceUnavailableException('登录会话服务暂时不可用');
    }
  }

  async recordLoginFailure(account: string, ip: string) {
    try {
      const accountKey = this.loginFailureKey('account', `${ip}\0${account}`);
      const ipKey = this.loginFailureKey('ip', ip);
      const pipeline = this.redis.pipeline();
      pipeline.incr(accountKey).expire(accountKey, 5 * 60);
      pipeline.incr(ipKey).expire(ipKey, 5 * 60);
      await pipeline.exec();
    } catch {
      throw new ServiceUnavailableException('登录会话服务暂时不可用');
    }
  }

  async clearLoginFailures(account: string, ip: string) {
    await this.redis.del(this.loginFailureKey('account', `${ip}\0${account}`), this.loginFailureKey('ip', ip)).catch(() => undefined);
  }

  idleSeconds(portal: AuthPortal) {
    return portal === 'admin' ? this.adminIdleSeconds : this.tenantIdleSeconds;
  }

  private async read(id: string): Promise<StoredAuthSession | null> {
    try {
      const raw = await this.redis.get(this.key(id));
      return raw ? JSON.parse(raw) as StoredAuthSession : null;
    } catch {
      throw new ServiceUnavailableException('登录会话服务暂时不可用');
    }
  }

  private async set(session: StoredAuthSession) {
    const absoluteRemaining = Math.floor((new Date(session.absoluteExpiresAt).getTime() - Date.now()) / 1000);
    const ttl = Math.min(this.idleSeconds(session.portal), absoluteRemaining);
    if (ttl <= 0) throw new UnauthorizedException('登录已达最长使用时间，请重新登录');
    try {
      await this.redis.set(this.key(session.id), JSON.stringify(session), 'EX', ttl);
    } catch {
      throw new ServiceUnavailableException('登录会话服务暂时不可用');
    }
  }

  private key(id: string) {
    return `${this.prefix}:${id}`;
  }

  private loginFailureKey(scope: string, value: string) {
    const digest = createHash('sha256').update(value).digest('hex');
    return `${this.prefix}:login-failure:${scope}:${digest}`;
  }

  private boundedSeconds(key: string, fallback: number, min: number, max: number) {
    const value = Number(this.config.get<string>(key, String(fallback)));
    return Number.isFinite(value) ? Math.max(min, Math.min(max, Math.floor(value))) : fallback;
  }

  async onModuleDestroy() {
    await this.redis.quit().catch(() => undefined);
  }
}

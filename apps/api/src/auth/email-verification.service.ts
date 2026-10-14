import { HttpException, HttpStatus, Injectable, OnModuleDestroy, ServiceUnavailableException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { createHash, randomInt } from 'crypto';
import { readFileSync } from 'fs';
import Redis from 'ioredis';
import nodemailer, { Transporter } from 'nodemailer';

export type EmailCodePurpose = 'register' | 'reset' | 'bind_email' | 'bind_phone' | 'login';

@Injectable()
export class EmailVerificationService implements OnModuleDestroy {
  private readonly redis: Redis;
  private readonly prefix: string;
  private readonly transporter: Transporter;
  private readonly from: string;
  private readonly ttlSeconds: number;
  private readonly cooldownSeconds: number;
  private readonly configured: boolean;

  constructor(private readonly config: ConfigService) {
    this.prefix = this.config.get<string>('EMAIL_CODE_PREFIX', 'weikong:email-code');
    this.ttlSeconds = this.boundedNumber('EMAIL_CODE_TTL_SECONDS', 300, 60, 900);
    this.cooldownSeconds = this.boundedNumber('EMAIL_CODE_COOLDOWN_SECONDS', 60, 30, 300);
    this.from = this.config.get<string>('MAIL_FROM', '').trim();
    const host = this.config.get<string>('MAIL_HOST', '').trim();
    const user = this.config.get<string>('MAIL_USER', '').trim();
    const password = this.readSecret('MAIL_PASSWORD', 'MAIL_PASSWORD_FILE');
    this.configured = Boolean(host && user && password && this.from);
    this.redis = new Redis({
      host: this.config.get<string>('REDIS_HOST', '127.0.0.1'),
      port: this.config.get<number>('REDIS_PORT', 6379),
      password: this.config.get<string>('REDIS_PASSWORD') || undefined,
      db: this.config.get<number>('REDIS_DB', 0),
      maxRetriesPerRequest: 1,
    });
    this.redis.on('error', () => undefined);
    this.transporter = nodemailer.createTransport({
      host,
      port: this.config.get<number>('MAIL_PORT', 465),
      secure: this.config.get<string>('MAIL_SECURE', 'true').toLowerCase() !== 'false',
      auth: {
        user,
        pass: password,
      },
      connectionTimeout: 10_000,
      greetingTimeout: 10_000,
      socketTimeout: 15_000,
    });
  }

  async send(email: string, purpose: EmailCodePurpose, ip: string) {
    this.assertConfigured();
    const normalizedEmail = email.trim().toLowerCase();
    const emailDigest = this.digest(normalizedEmail);
    const ipDigest = this.digest(ip || 'unknown');
    const cooldownKey = `${this.prefix}:cooldown:${purpose}:${emailDigest}`;
    const codeKey = `${this.prefix}:value:${purpose}:${emailDigest}`;
    const attemptsKey = `${this.prefix}:attempts:${purpose}:${emailDigest}`;

    try {
      const cooldownSet = await this.redis.set(cooldownKey, '1', 'EX', this.cooldownSeconds, 'NX');
      if (!cooldownSet) throw new HttpException(`请在 ${this.cooldownSeconds} 秒后重新发送`, HttpStatus.TOO_MANY_REQUESTS);
      await this.enforceHourlyLimit(`${this.prefix}:limit:email:${emailDigest}`, 5);
      await this.enforceHourlyLimit(`${this.prefix}:limit:ip:${ipDigest}`, 20);

      const code = String(randomInt(100000, 1000000));
      await this.redis
        .multi()
        .set(codeKey, this.digest(code), 'EX', this.ttlSeconds)
        .del(attemptsKey)
        .exec();

      try {
        await this.transporter.sendMail({
          from: this.from,
          to: normalizedEmail,
          subject: `${this.actionLabel(purpose)}验证码`,
          text: `您的验证码是 ${code}，${Math.floor(this.ttlSeconds / 60)} 分钟内有效。请勿向任何人泄露此验证码。`,
          html: this.messageHtml(code, purpose),
        });
      } catch (error) {
        await this.redis.del(codeKey, attemptsKey, cooldownKey).catch(() => undefined);
        const detail = error instanceof Error ? error.message : String(error);
        console.error(`[email] verification email delivery failed: ${detail}`);
        throw new ServiceUnavailableException('验证码邮件发送失败，请稍后重试');
      }
    } catch (error) {
      if (error instanceof HttpException) throw error;
      throw new ServiceUnavailableException('验证码服务暂时不可用');
    }

    return {
      message: '验证码已发送',
      expiresInSeconds: this.ttlSeconds,
      retryAfterSeconds: this.cooldownSeconds,
      email: this.maskEmail(normalizedEmail),
    };
  }

  async verifyAndConsume(email: string, purpose: EmailCodePurpose, code: string) {
    const emailDigest = this.digest(email.trim().toLowerCase());
    const codeKey = `${this.prefix}:value:${purpose}:${emailDigest}`;
    const attemptsKey = `${this.prefix}:attempts:${purpose}:${emailDigest}`;
    try {
      const result = Number(await this.redis.eval(
        `local stored = redis.call('GET', KEYS[1])
         if not stored then return 0 end
         if stored == ARGV[1] then
           redis.call('DEL', KEYS[1], KEYS[2])
           return 1
         end
         local attempts = redis.call('INCR', KEYS[2])
         if attempts == 1 then redis.call('EXPIRE', KEYS[2], ARGV[2]) end
         if attempts >= 5 then
           redis.call('DEL', KEYS[1], KEYS[2])
           return -2
         end
         return -1`,
        2,
        codeKey,
        attemptsKey,
        this.digest(code.trim()),
        this.ttlSeconds,
      ));
      if (result === 1) return;
      if (result === -1) throw new HttpException('验证码错误', HttpStatus.BAD_REQUEST);
      if (result === -2) throw new HttpException('验证码错误次数过多，请重新获取', HttpStatus.BAD_REQUEST);
      throw new HttpException('验证码已失效，请重新获取', HttpStatus.BAD_REQUEST);
    } catch (error) {
      if (error instanceof HttpException) throw error;
      throw new ServiceUnavailableException('验证码服务暂时不可用');
    }
  }

  private async enforceHourlyLimit(key: string, limit: number) {
    const count = await this.redis.incr(key);
    if (count === 1) await this.redis.expire(key, 60 * 60);
    if (count > limit) throw new HttpException('验证码发送过于频繁，请稍后再试', HttpStatus.TOO_MANY_REQUESTS);
  }

  private assertConfigured() {
    if (!this.configured) throw new ServiceUnavailableException('邮件服务尚未配置');
  }

  private readSecret(valueKey: string, fileKey: string) {
    const direct = this.config.get<string>(valueKey, '').trim();
    if (direct) return direct;
    const path = this.config.get<string>(fileKey, '').trim();
    if (!path) return '';
    try {
      return readFileSync(path, 'utf8').trim();
    } catch {
      return '';
    }
  }

  private messageHtml(code: string, purpose: EmailCodePurpose) {
    const action = this.actionLabel(purpose);
    return `<!doctype html><html><body style="margin:0;background:#f5f7fb;font-family:Arial,'Microsoft YaHei',sans-serif;color:#172033"><div style="max-width:520px;margin:36px auto;padding:32px;background:#fff;border-radius:12px"><h2 style="margin:0 0 12px">微控物联云平台</h2><p>您正在${action}，验证码为：</p><div style="margin:24px 0;font-size:32px;font-weight:700;letter-spacing:8px;color:#1677ff">${code}</div><p style="color:#687386">验证码将在 ${Math.floor(this.ttlSeconds / 60)} 分钟后失效，请勿向任何人泄露。</p><p style="color:#98a1b2;font-size:12px">如果不是您本人操作，请忽略此邮件。</p></div></body></html>`;
  }

  private actionLabel(purpose: EmailCodePurpose) {
    return {
      register: '注册微控物联云平台',
      reset: '重置微控物联云平台密码',
      bind_email: '绑定微控物联云平台邮箱',
      bind_phone: '修改微控物联云平台绑定手机号',
      login: '登录微控物联云平台',
    }[purpose];
  }

  private maskEmail(email: string) {
    const [local, domain] = email.split('@');
    const visible = local.length <= 2 ? `${local[0] || '*'}*` : `${local.slice(0, 2)}***${local.slice(-1)}`;
    return `${visible}@${domain}`;
  }

  private digest(value: string) {
    return createHash('sha256').update(value).digest('hex');
  }

  private boundedNumber(key: string, fallback: number, min: number, max: number) {
    const value = Number(this.config.get<string>(key, String(fallback)));
    return Number.isFinite(value) ? Math.max(min, Math.min(max, Math.floor(value))) : fallback;
  }

  async onModuleDestroy() {
    await this.redis.quit().catch(() => undefined);
  }
}

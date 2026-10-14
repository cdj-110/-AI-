import { HttpException, HttpStatus, Injectable, OnModuleDestroy, ServiceUnavailableException, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { createHash, randomBytes, randomInt } from 'crypto';
import { readFileSync } from 'fs';
import { join } from 'path';
import Redis from 'ioredis';
import { CaptchaTrailPointDto } from './dto/captcha.dto';

type StoredChallenge = {
  targetX: number;
  ipHash: string;
};

type CaptchaTiming = {
  durationMs: number;
  percentile: number | null;
};

const IMAGE_WIDTH = 320;
const IMAGE_HEIGHT = 150;
const PIECE_SIZE = 48;
const MAX_OFFSET = IMAGE_WIDTH - PIECE_SIZE;
const INDUSTRIAL_PHOTO_FILES = [
  'captcha-smart-factory.jpg',
  'captcha-pipeline.jpg',
  'captcha-robot-arm.jpg',
  'captcha-control-room.jpg',
];

@Injectable()
export class CaptchaService implements OnModuleDestroy {
  private readonly redis: Redis;
  private readonly prefix: string;
  private readonly industrialPhotos: string[];

  constructor(private readonly config: ConfigService) {
    this.prefix = this.config.get<string>('AUTH_CAPTCHA_PREFIX', 'weikong:auth-captcha');
    this.industrialPhotos = INDUSTRIAL_PHOTO_FILES.map((file) => {
      const image = readFileSync(join(__dirname, 'assets', file));
      return `data:image/jpeg;base64,${image.toString('base64')}`;
    });
    this.redis = new Redis({
      host: this.config.get<string>('REDIS_HOST', '127.0.0.1'),
      port: this.config.get<number>('REDIS_PORT', 6379),
      password: this.config.get<string>('REDIS_PASSWORD') || undefined,
      db: this.config.get<number>('REDIS_DB', 0),
      maxRetriesPerRequest: 1,
    });
    this.redis.on('error', () => undefined);
  }

  async create(ip: string) {
    const ipHash = this.digest(ip || 'unknown');
    await this.assertIssueAllowed(ipHash);
    const challengeId = randomBytes(24).toString('base64url');
    const targetX = randomInt(54, MAX_OFFSET - 8);
    const targetY = randomInt(38, IMAGE_HEIGHT - PIECE_SIZE - 18);
    const photo = this.industrialPhotos[randomInt(0, this.industrialPhotos.length)];
    const stored: StoredChallenge = { targetX, ipHash };
    try {
      await this.redis.set(this.challengeKey(challengeId), JSON.stringify(stored), 'EX', 120);
    } catch {
      throw new ServiceUnavailableException('安全验证服务暂时不可用');
    }
    return {
      challengeId,
      imageWidth: IMAGE_WIDTH,
      pieceSize: PIECE_SIZE,
      pieceY: targetY,
      background: this.svgData(this.backgroundPhotoSvg(photo, targetX, targetY)),
      piece: this.svgData(this.piecePhotoSvg(photo, targetX, targetY)),
      expiresIn: 120,
    };
  }

  async verify(challengeId: string, offsetX: number, trail: CaptchaTrailPointDto[], ip: string) {
    let raw: string | null;
    try {
      raw = await this.redis.getdel(this.challengeKey(challengeId));
    } catch {
      throw new ServiceUnavailableException('安全验证服务暂时不可用');
    }
    if (!raw) throw new UnauthorizedException('验证已失效，请刷新后重试');
    const challenge = JSON.parse(raw) as StoredChallenge;
    if (challenge.ipHash !== this.digest(ip || 'unknown')) throw new UnauthorizedException('验证环境已变化，请重新验证');
    if (!this.validTrail(trail, offsetX) || Math.abs(challenge.targetX - offsetX) > 6) {
      throw new UnauthorizedException('拼图位置不正确，请重试');
    }
    const token = randomBytes(32).toString('base64url');
    try {
      await this.redis.set(this.proofKey(token), challenge.ipHash, 'EX', 120);
    } catch {
      throw new ServiceUnavailableException('安全验证服务暂时不可用');
    }
    const durationMs = trail[trail.length - 1].t;
    const timing = await this.recordTiming(durationMs);
    return { captchaToken: token, expiresIn: 120, ...timing };
  }

  async consume(token: string, ip: string) {
    let ipHash: string | null;
    try {
      ipHash = await this.redis.getdel(this.proofKey(token));
    } catch {
      throw new ServiceUnavailableException('安全验证服务暂时不可用');
    }
    if (!ipHash || ipHash !== this.digest(ip || 'unknown')) {
      throw new UnauthorizedException('请重新完成安全验证');
    }
  }

  private validTrail(trail: CaptchaTrailPointDto[], offsetX: number) {
    if (!Array.isArray(trail) || trail.length < 3 || trail.length > 160) return false;
    const first = trail[0];
    const last = trail[trail.length - 1];
    if (first.t !== 0 || last.t < 250 || last.t > 15_000) return false;
    if (Math.abs(last.x - offsetX) > 3) return false;
    return trail.every((point, index) => index === 0 || point.t >= trail[index - 1].t);
  }

  private async assertIssueAllowed(ipHash: string) {
    const key = `${this.prefix}:issue:${ipHash}`;
    try {
      const count = await this.redis.incr(key);
      if (count === 1) await this.redis.expire(key, 60);
      if (count > 30) throw new HttpException('安全验证请求过于频繁，请稍后重试', HttpStatus.TOO_MANY_REQUESTS);
    } catch (error) {
      if (error instanceof HttpException) throw error;
      throw new ServiceUnavailableException('安全验证服务暂时不可用');
    }
  }

  private async recordTiming(durationMs: number): Promise<CaptchaTiming> {
    const key = `${this.prefix}:stats:durations`;
    try {
      const pipeline = this.redis.pipeline();
      pipeline.lpush(key, String(durationMs));
      pipeline.ltrim(key, 0, 999);
      pipeline.expire(key, 90 * 24 * 60 * 60);
      await pipeline.exec();
      const samples = (await this.redis.lrange(key, 0, 999))
        .map((value) => Number(value))
        .filter((value) => Number.isFinite(value) && value >= 250 && value <= 15_000);
      if (!samples.length) return { durationMs, percentile: null };
      const slower = samples.filter((value) => value > durationMs).length;
      const equal = samples.filter((value) => value === durationMs).length;
      const percentile = Math.round(((slower + equal * 0.5) / samples.length) * 100);
      return { durationMs, percentile: Math.max(1, Math.min(99, percentile)) };
    } catch {
      // 统计信息不参与安全判定，Redis 短暂异常时仍允许已通过的验证继续登录。
      return { durationMs, percentile: null };
    }
  }

  private backgroundPhotoSvg(photo: string, targetX: number, targetY: number) {
    const shape = this.puzzlePath(targetX, targetY);
    return `<svg xmlns="http://www.w3.org/2000/svg" width="${IMAGE_WIDTH}" height="${IMAGE_HEIGHT}" viewBox="0 0 ${IMAGE_WIDTH} ${IMAGE_HEIGHT}"><defs><linearGradient id="shade" x1="0" y1="0" x2="0" y2="1"><stop stop-color="#07152d" stop-opacity=".03"/><stop offset="1" stop-color="#07152d" stop-opacity=".18"/></linearGradient><filter id="s"><feDropShadow dx="0" dy="1" stdDeviation="2" flood-opacity=".42"/></filter></defs><image href="${photo}" width="${IMAGE_WIDTH}" height="${IMAGE_HEIGHT}" preserveAspectRatio="xMidYMid slice"/><rect width="${IMAGE_WIDTH}" height="${IMAGE_HEIGHT}" fill="url(#shade)"/><path d="${shape}" fill="#07152d" opacity=".58" stroke="#fff" stroke-width="2" stroke-opacity=".94" filter="url(#s)"/></svg>`;
  }

  private piecePhotoSvg(photo: string, targetX: number, targetY: number) {
    const shape = this.puzzlePath(0, 0);
    return `<svg xmlns="http://www.w3.org/2000/svg" width="${PIECE_SIZE}" height="${PIECE_SIZE}" viewBox="0 0 ${PIECE_SIZE} ${PIECE_SIZE}"><defs><clipPath id="piece"><path d="${shape}"/></clipPath><filter id="s"><feDropShadow dx="0" dy="2" stdDeviation="2" flood-opacity=".48"/></filter></defs><g clip-path="url(#piece)" filter="url(#s)"><image href="${photo}" x="${-targetX}" y="${-targetY}" width="${IMAGE_WIDTH}" height="${IMAGE_HEIGHT}" preserveAspectRatio="xMidYMid slice"/></g><path d="${shape}" fill="none" stroke="#fff" stroke-width="2"/></svg>`;
  }

  private puzzlePath(x: number, y: number) {
    const right = x + PIECE_SIZE;
    const bottom = y + PIECE_SIZE;
    return `M${x},${y} H${x + 17} C${x + 13},${y + 12} ${x + 35},${y + 12} ${x + 31},${y} H${right} V${y + 17} C${right - 12},${y + 13} ${right - 12},${y + 35} ${right},${y + 31} V${bottom} H${x} V${y + 31} C${x + 12},${y + 35} ${x + 12},${y + 13} ${x},${y + 17} Z`;
  }

  private svgData(svg: string) {
    return `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`;
  }

  private digest(value: string) {
    return createHash('sha256').update(value).digest('hex');
  }

  private challengeKey(id: string) {
    return `${this.prefix}:challenge:${id}`;
  }

  private proofKey(token: string) {
    return `${this.prefix}:proof:${this.digest(token)}`;
  }

  async onModuleDestroy() {
    await this.redis.quit().catch(() => undefined);
  }
}

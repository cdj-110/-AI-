import { Injectable, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { PassportStrategy } from '@nestjs/passport';
import { ExtractJwt, Strategy } from 'passport-jwt';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { AuthSessionService } from './auth-session.service';

@Injectable()
export class JwtStrategy extends PassportStrategy(Strategy) {
  constructor(configService: ConfigService, private readonly prisma: PrismaService, private readonly sessions: AuthSessionService) {
    super({
      jwtFromRequest: ExtractJwt.fromAuthHeaderAsBearerToken(),
      ignoreExpiration: false,
      secretOrKey: configService.get<string>('JWT_SECRET', 'weikong_iot_secret'),
    });
  }

  async validate(payload: AuthUser) {
    if (!payload.sub || !payload.sid) throw new UnauthorizedException('登录已失效，请重新登录');
    await this.sessions.validate(payload.sid, payload.sub, payload.portal);
    const user = await this.prisma.user.findUnique({
      where: { id: payload.sub },
      include: { tenant: { select: { status: true } } },
    });
    if (!user || user.status !== 'ACTIVE') throw new UnauthorizedException('账号已停用或不存在');
    if (!payload.uv || payload.uv !== user.updatedAt.getTime()) throw new UnauthorizedException('账号信息已变更，请重新登录');
    if (user.role !== 'SUPER_ADMIN' && user.tenant && user.tenant.status !== 'ACTIVE') {
      throw new UnauthorizedException('您所属的租户已被停用，请联系平台管理员');
    }
    if (user.role !== 'SUPER_ADMIN' && (!user.tenantId || !user.tenant)) {
      throw new UnauthorizedException('账号未关联租户');
    }
    if (payload.portal === 'admin' && user.role !== 'SUPER_ADMIN') throw new UnauthorizedException('账号与当前平台不匹配');
    if (payload.portal === 'tenant' && user.role === 'SUPER_ADMIN') throw new UnauthorizedException('账号与当前平台不匹配');
    return {
      sub: user.id,
      sid: payload.sid,
      uv: user.updatedAt.getTime(),
      username: user.username,
      role: user.role,
      tenantId: user.tenantId,
      permissions: user.permissions,
      deviceScopeMode: user.deviceScopeMode as 'ALL' | 'CUSTOM',
      portal: payload.portal,
    };
  }
}

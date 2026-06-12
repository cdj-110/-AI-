import { BadRequestException, Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { User } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { PrismaService } from '../prisma/prisma.service';
import { LoginDto } from './dto/login.dto';
import { RegisterDto } from './dto/register.dto';
import { ResetPasswordDto } from './dto/reset-password.dto';

@Injectable()
export class AuthService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly jwtService: JwtService,
  ) {}

  async login(dto: LoginDto, meta: { ip: string; userAgent?: string | string[] }) {
    const account = dto.username.trim();
    const user = await this.prisma.user.findFirst({
      where: {
        OR: [
          { username: account },
          { phone: account },
          { email: account },
        ],
      },
    });

    if (!user || user.status !== 'ACTIVE' || !(await bcrypt.compare(dto.password, user.password))) {
      await this.recordLoginLog({
        username: account,
        user,
        ip: meta.ip,
        userAgent: meta.userAgent,
        success: false,
        reason: !user ? '账号不存在' : user.status !== 'ACTIVE' ? '账号已停用' : '密码错误',
      });
      throw new UnauthorizedException('账号或密码错误');
    }

    await this.recordLoginLog({
      username: account,
      user,
      ip: meta.ip,
      userAgent: meta.userAgent,
      success: true,
    });
    return { accessToken: this.signToken(user), user: this.toPublicUser(user) };
  }

  async profile(id: string) {
    const user = await this.prisma.user.findUnique({ where: { id } });
    if (!user) throw new UnauthorizedException('用户不存在');
    return this.toPublicUser(user);
  }

  async register(dto: RegisterDto) {
    this.validateCodeAndPasswords(dto.code, dto.password, dto.confirmPassword);
    const username = dto.username.trim();
    const phone = dto.phone?.trim() || undefined;
    const email = dto.email?.trim() || undefined;
    const exists = await this.prisma.user.findFirst({
      where: {
        OR: [
          { username },
          ...(phone ? [{ phone }] : []),
          ...(email ? [{ email }] : []),
        ],
      },
    });
    if (exists) throw new BadRequestException('用户名已存在');
    const password = await bcrypt.hash(dto.password, 10);
    const user = await this.prisma.$transaction(async (tx) => {
      const tenant = await tx.tenant.create({ data: { name: `${username}的组织` } });
      return tx.user.create({
        data: {
          username,
          password,
          phone,
          email,
          role: 'TENANT_ADMIN',
          tenantId: tenant.id,
        },
      });
    });
    return { accessToken: this.signToken(user), user: this.toPublicUser(user) };
  }

  async resetPassword(dto: ResetPasswordDto) {
    this.validateCodeAndPasswords(dto.code, dto.password, dto.confirmPassword);
    const user = await this.prisma.user.findUnique({ where: { username: dto.username } });
    if (!user) throw new BadRequestException('用户不存在');
    await this.prisma.user.update({
      where: { id: user.id },
      data: { password: await bcrypt.hash(dto.password, 10) },
    });
    return { message: '密码重置成功' };
  }

  private validateCodeAndPasswords(code: string, password: string, confirmPassword: string) {
    if (code !== '123456') throw new BadRequestException('验证码错误');
    if (password !== confirmPassword) throw new BadRequestException('两次输入的密码不一致');
  }

  private signToken(user: User) {
    return this.jwtService.sign({
      sub: user.id,
      username: user.username,
      role: user.role,
      tenantId: user.tenantId,
    });
  }

  private async recordLoginLog(options: {
    username: string;
    user?: User | null;
    ip: string;
    userAgent?: string | string[];
    success: boolean;
    reason?: string;
  }) {
    await this.prisma.loginLog.create({
      data: {
        username: options.username,
        userId: options.user?.id,
        tenantId: options.user?.tenantId,
        ip: options.ip || 'unknown',
        userAgent: Array.isArray(options.userAgent) ? options.userAgent.join(' ') : options.userAgent,
        success: options.success,
        reason: options.reason,
      },
    });
  }

  private toPublicUser(user: User) {
    const { password: _password, ...publicUser } = user;
    return publicUser;
  }
}

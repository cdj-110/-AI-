import { BadRequestException, HttpException, HttpStatus, Injectable, NotFoundException, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { Prisma, User } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { mkdir, rename, unlink, writeFile } from 'node:fs/promises';
import { basename, join, resolve } from 'node:path';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { PrismaService } from '../prisma/prisma.service';
import { LoginDto } from './dto/login.dto';
import { RegisterDto } from './dto/register.dto';
import { ResetPasswordDto } from './dto/reset-password.dto';
import { AuthPortal, AuthSessionService } from './auth-session.service';
import { CaptchaService } from './captcha.service';
import { SendEmailCodeDto } from './dto/send-email-code.dto';
import { EmailVerificationService } from './email-verification.service';
import { AccountContactCodeDto, UpdateAccountContactDto } from './dto/account-contact.dto';
import { UpdateProfileDto } from './dto/update-profile.dto';
import { LoginWithCodeDto, SendLoginCodeDto } from './dto/login-code.dto';

@Injectable()
export class AuthService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly jwtService: JwtService,
    private readonly sessions: AuthSessionService,
    private readonly captcha: CaptchaService,
    private readonly emailVerification: EmailVerificationService,
  ) {}

  async login(
    dto: LoginDto,
    meta: { ip: string; userAgent?: string | string[] },
    portal: 'admin' | 'tenant' | 'legacy' = 'legacy',
  ) {
    const account = this.normalizeLoginCredential(dto.username);
    const ip = meta.ip || 'unknown';
    try {
      await this.sessions.assertLoginAllowed(account, ip);
    } catch (error) {
      await this.recordLoginLog({ username: account, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: '登录失败次数过多' });
      throw error;
    }
    const captchaRequired = await this.sessions.requiresCaptcha(account, ip);
    if (captchaRequired && !dto.captchaToken) {
      throw new HttpException('请完成安全验证后重试', HttpStatus.PRECONDITION_REQUIRED);
    }
    if (dto.captchaToken) await this.captcha.consume(dto.captchaToken, ip);
    const user = await this.findByLoginCredential(account);

    const passwordMatches = Boolean(user && await bcrypt.compare(dto.password, user.password));
    if (!user || user.status !== 'ACTIVE' || !passwordMatches) {
      await this.sessions.recordLoginFailure(account, ip);
      await this.recordLoginLog({
        username: account,
        user,
        ip: meta.ip,
        userAgent: meta.userAgent,
        success: false,
        reason: !user ? '账号不存在' : user.status !== 'ACTIVE' ? '账号已停用' : '密码错误',
      });
      throw new UnauthorizedException('邮箱、手机号或密码错误');
    }

    await this.sessions.clearLoginFailures(account, ip);

    if (user.role !== 'SUPER_ADMIN' && !user.tenantId) {
      await this.recordLoginLog({
        username: account,
        user,
        ip: meta.ip,
        userAgent: meta.userAgent,
        success: false,
        reason: '账号未关联租户',
      });
      throw new UnauthorizedException('账号未关联租户，请联系管理员');
    }

    if (user.role !== 'SUPER_ADMIN' && user.tenantId) {
      const tenant = await this.prisma.tenant.findUnique({ where: { id: user.tenantId }, select: { status: true } });
      if (!tenant || tenant.status !== 'ACTIVE') {
        await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: '租户已停用或不存在' });
        throw new UnauthorizedException('您所属的租户已被停用，请联系平台管理员');
      }
    }

    const portalAllowed = portal === 'legacy'
      || (portal === 'admin' && user.role === 'SUPER_ADMIN')
      || (portal === 'tenant' && ['TENANT_ADMIN', 'TENANT_USER'].includes(user.role));
    if (!portalAllowed) {
      await this.recordLoginLog({
        username: account,
        user,
        ip: meta.ip,
        userAgent: meta.userAgent,
        success: false,
        reason: portal === 'admin' ? '非公司管理账号' : '非租户账号',
      });
      throw new UnauthorizedException(portal === 'admin'
        ? '该账号不能登录公司管理平台'
        : '该账号不能登录租户物联网平台');
    }

    await this.recordLoginLog({
      username: account,
      user,
      ip: meta.ip,
      userAgent: meta.userAgent,
      success: true,
    });
    return { ...(await this.signToken(user, portal)), user: this.toPublicUser(user) };
  }

  async sendLoginCode(dto: SendLoginCodeDto, ip: string, portal: 'admin' | 'tenant') {
    const account = this.normalizeLoginCredential(dto.username);
    await this.sessions.assertLoginAllowed(account, ip || 'unknown');
    await this.captcha.consume(dto.captchaToken, ip || 'unknown');
    const user = await this.findByLoginCredential(account);
    const portalAllowed = user && (portal === 'admin'
      ? user.role === 'SUPER_ADMIN'
      : ['TENANT_ADMIN', 'TENANT_USER'].includes(user.role));
    const tenantAllowed = !user || user.role === 'SUPER_ADMIN' || !user.tenantId
      ? true
      : (await this.prisma.tenant.findUnique({ where: { id: user.tenantId }, select: { status: true } }))?.status === 'ACTIVE';

    // 不向未认证访问者泄露账号、角色或邮箱绑定情况。
    if (!user || user.status !== 'ACTIVE' || !portalAllowed || !tenantAllowed || !user.email) {
      return {
        message: '如果账号有效，验证码将发送至绑定邮箱',
        expiresInSeconds: 300,
        retryAfterSeconds: 60,
      };
    }
    return this.emailVerification.send(user.email, 'login', ip);
  }

  async loginWithCode(
    dto: LoginWithCodeDto,
    meta: { ip: string; userAgent?: string | string[] },
    portal: 'admin' | 'tenant',
  ) {
    const account = this.normalizeLoginCredential(dto.username);
    const ip = meta.ip || 'unknown';
    await this.sessions.assertLoginAllowed(account, ip);
    const user = await this.findByLoginCredential(account);

    if (!user || user.status !== 'ACTIVE' || !user.email) {
      await this.sessions.recordLoginFailure(account, ip);
      await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: !user ? '账号不存在' : user.status !== 'ACTIVE' ? '账号已停用' : '账号未绑定邮箱' });
      throw new UnauthorizedException('账号或验证码错误');
    }

    try {
      await this.emailVerification.verifyAndConsume(user.email, 'login', dto.code);
    } catch (error) {
      await this.sessions.recordLoginFailure(account, ip);
      await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: '验证码错误或失效' });
      throw error;
    }

    if (user.role !== 'SUPER_ADMIN' && !user.tenantId) {
      await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: '账号未关联租户' });
      throw new UnauthorizedException('账号未关联租户，请联系管理员');
    }
    if (user.role !== 'SUPER_ADMIN' && user.tenantId) {
      const tenant = await this.prisma.tenant.findUnique({ where: { id: user.tenantId }, select: { status: true } });
      if (!tenant || tenant.status !== 'ACTIVE') {
        await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: '租户已停用或不存在' });
        throw new UnauthorizedException('您所属的租户已被停用，请联系平台管理员');
      }
    }

    const portalAllowed = portal === 'admin'
      ? user.role === 'SUPER_ADMIN'
      : ['TENANT_ADMIN', 'TENANT_USER'].includes(user.role);
    if (!portalAllowed) {
      await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: false, reason: portal === 'admin' ? '非公司管理账号' : '非租户账号' });
      throw new UnauthorizedException(portal === 'admin' ? '该账号不能登录公司管理平台' : '该账号不能登录租户物联网平台');
    }

    await this.sessions.clearLoginFailures(account, ip);
    await this.recordLoginLog({ username: account, user, ip: meta.ip, userAgent: meta.userAgent, success: true, reason: '验证码登录' });
    return { ...(await this.signToken(user, portal)), user: this.toPublicUser(user) };
  }

  async profile(id: string) {
    const user = await this.prisma.user.findUnique({ where: { id } });
    if (!user) throw new UnauthorizedException('用户不存在');
    return this.toPublicUser(user);
  }

  async updateProfile(actor: AuthUser, dto: UpdateProfileDto) {
    const username = dto.username.trim();
    if (!username) throw new BadRequestException('请输入用户名称');
    const user = await this.prisma.user.update({
      where: { id: actor.sub },
      // nickname 是历史展示字段，同步更新可保证平台各处立即显示新的用户名称。
      data: { username, nickname: username },
    });
    return this.profileUpdateResult(user, actor);
  }

  async updateAvatar(
    actor: AuthUser,
    file?: { buffer: Buffer; mimetype: string; size: number },
  ) {
    if (!file?.buffer?.length) throw new BadRequestException('请选择头像图片');
    const format = this.detectAvatarFormat(file.buffer);
    if (!format) throw new BadRequestException('头像仅支持 JPG、PNG 或 WebP 图片');

    const directory = this.avatarDirectory();
    await mkdir(directory, { recursive: true });
    const filename = `${actor.sub}.${format.extension}`;
    const absolutePath = join(directory, filename);
    const temporaryPath = join(directory, `.${actor.sub}-${Date.now()}.tmp`);
    await writeFile(temporaryPath, file.buffer, { mode: 0o640 });
    await unlink(absolutePath).catch(() => undefined);
    await rename(temporaryPath, absolutePath);

    const avatarUrl = `/api/auth/avatars/${actor.sub}/${filename}?v=${Date.now()}`;
    const user = await this.prisma.user.update({ where: { id: actor.sub }, data: { avatarUrl } });
    await Promise.all(
      ['jpg', 'png', 'webp']
        .filter((extension) => extension !== format.extension)
        .map((extension) => unlink(join(directory, `${actor.sub}.${extension}`)).catch(() => undefined)),
    );
    return this.profileUpdateResult(user, actor);
  }

  async avatarFile(userId: string) {
    const user = await this.prisma.user.findUnique({ where: { id: userId }, select: { avatarUrl: true } });
    if (!user?.avatarUrl) throw new NotFoundException('头像不存在');
    const filename = basename(user.avatarUrl.split('?')[0]);
    const extension = filename.split('.').pop()?.toLowerCase();
    const mimeType = extension === 'png' ? 'image/png' : extension === 'webp' ? 'image/webp' : 'image/jpeg';
    return { absolutePath: resolve(this.avatarDirectory(), filename), mimeType };
  }

  private avatarDirectory() {
    return resolve(process.env.AVATAR_DIR || join(process.cwd(), 'storage', 'avatars'));
  }

  private detectAvatarFormat(buffer: Buffer) {
    if (buffer.length >= 8 && buffer.subarray(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]))) {
      return { extension: 'png', mimeType: 'image/png' } as const;
    }
    if (buffer.length >= 3 && buffer[0] === 0xff && buffer[1] === 0xd8 && buffer[2] === 0xff) {
      return { extension: 'jpg', mimeType: 'image/jpeg' } as const;
    }
    if (buffer.length >= 12 && buffer.subarray(0, 4).toString('ascii') === 'RIFF' && buffer.subarray(8, 12).toString('ascii') === 'WEBP') {
      return { extension: 'webp', mimeType: 'image/webp' } as const;
    }
    return null;
  }

  private profileUpdateResult(user: User, actor: AuthUser) {
    const portal = actor.portal ?? 'tenant';
    const accessToken = this.jwtService.sign({
      sub: user.id,
      sid: actor.sid,
      uv: user.updatedAt.getTime(),
      username: user.username,
      role: user.role,
      tenantId: user.tenantId,
      portal,
    });
    return { accessToken, user: this.toPublicUser(user) };
  }

  async sendAccountContactCode(userId: string, dto: AccountContactCodeDto, ip: string) {
    const user = await this.prisma.user.findUnique({ where: { id: userId } });
    if (!user) throw new UnauthorizedException('用户不存在');

    if (dto.type === 'email') {
      const email = this.normalizeEmail(dto.value);
      if (email === user.email) throw new BadRequestException('该邮箱已是当前绑定邮箱');
      await this.assertContactAvailable('email', email, user.id);
      return this.emailVerification.send(email, 'bind_email', ip);
    }

    const phone = this.normalizePhone(dto.value);
    if (phone === user.phone) throw new BadRequestException('该手机号已是当前绑定手机号');
    await this.assertContactAvailable('phone', phone, user.id);
    if (!user.email) throw new BadRequestException('请先绑定邮箱，再修改手机号');
    const result = await this.emailVerification.send(user.email, 'bind_phone', ip);
    return { ...result, message: '验证码已发送至当前绑定邮箱' };
  }

  async updateAccountContact(actor: AuthUser, dto: UpdateAccountContactDto) {
    const user = await this.prisma.user.findUnique({ where: { id: actor.sub } });
    if (!user) throw new UnauthorizedException('用户不存在');

    let data: Prisma.UserUpdateInput;
    if (dto.type === 'email') {
      const email = this.normalizeEmail(dto.value);
      if (email === user.email) throw new BadRequestException('该邮箱已是当前绑定邮箱');
      await this.assertContactAvailable('email', email, user.id);
      await this.emailVerification.verifyAndConsume(email, 'bind_email', dto.code);
      data = { email };
    } else {
      const phone = this.normalizePhone(dto.value);
      if (phone === user.phone) throw new BadRequestException('该手机号已是当前绑定手机号');
      await this.assertContactAvailable('phone', phone, user.id);
      if (!user.email) throw new BadRequestException('请先绑定邮箱，再修改手机号');
      await this.emailVerification.verifyAndConsume(user.email, 'bind_phone', dto.code);
      data = { phone };
    }

    try {
      await this.prisma.user.update({ where: { id: user.id }, data });
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2002') {
        throw new BadRequestException(dto.type === 'email' ? '该邮箱已被其他账号使用' : '该手机号已被其他账号使用');
      }
      throw error;
    }
    await this.sessions.revoke(actor.sid);
    return { message: '登录凭证已更新，请重新登录', requiresRelogin: true };
  }

  async register(dto: RegisterDto) {
    this.validatePasswords(dto.password, dto.confirmPassword);
    const username = dto.username.trim();
    const email = dto.email.trim().toLowerCase();
    if (!username) throw new BadRequestException('请输入用户名称');
    const exists = await this.prisma.user.findUnique({ where: { email } });
    if (exists) throw new BadRequestException('该邮箱已注册');
    await this.emailVerification.verifyAndConsume(email, 'register', dto.code);
    const password = await bcrypt.hash(dto.password, 10);
    const user = await this.prisma.$transaction(async (tx) => {
      const plan = await tx.subscriptionPlan.findFirst({ where: { enabled: true }, orderBy: [{ priceCents: 'asc' }, { sortOrder: 'asc' }, { createdAt: 'asc' }] });
      const now = new Date();
      const expiresAt = new Date(now); expiresAt.setUTCFullYear(expiresAt.getUTCFullYear() + 1);
      const graceEndsAt = new Date(expiresAt.getTime() + (plan?.gracePeriodDays ?? 7) * 86_400_000);
      const tenant = await tx.tenant.create({ data: {
        name: `${username}的组织`, subscriptionPlanId: plan?.id, subscriptionStartedAt: plan ? now : undefined,
        ...(plan ? { subscription: { create: {
          planId: plan.id, status: 'ACTIVE', billingCycle: 'YEARLY', startedAt: now, expiresAt, graceEndsAt,
          baseDeviceLimit: plan.deviceLimit, basePointLimit: plan.pointLimit,
          baseScadaProjectLimit: plan.scadaProjectLimit, baseUserLimit: plan.userLimit,
        } } } : {}),
      } });
      return tx.user.create({
        data: {
          username,
          password,
          email,
          role: 'TENANT_ADMIN',
          tenantId: tenant.id,
        },
      });
    });
    return { ...(await this.signToken(user)), user: this.toPublicUser(user) };
  }

  async resetPassword(dto: ResetPasswordDto) {
    this.validatePasswords(dto.password, dto.confirmPassword);
    const account = dto.username.trim();
    const user = await this.findByLoginCredential(account);
    if (!user) throw new BadRequestException('用户不存在');
    if (!user.email) throw new BadRequestException('该账号未绑定邮箱，请联系管理员');
    await this.emailVerification.verifyAndConsume(user.email, 'reset', dto.code);
    await this.prisma.user.update({
      where: { id: user.id },
      data: { password: await bcrypt.hash(dto.password, 10) },
    });
    return { message: '密码重置成功' };
  }

  async sendEmailCode(dto: SendEmailCodeDto, ip: string) {
    const target = dto.target.trim();
    if (dto.purpose === 'register') {
      const email = target.toLowerCase();
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) throw new BadRequestException('请输入正确的邮箱地址');
      const exists = await this.prisma.user.findFirst({ where: { email } });
      if (exists) throw new BadRequestException('该邮箱已注册');
      return this.emailVerification.send(email, 'register', ip);
    }

    const user = await this.findByLoginCredential(target);
    if (!user) throw new BadRequestException('用户不存在');
    if (!user.email) throw new BadRequestException('该账号未绑定邮箱，请联系管理员');
    return this.emailVerification.send(user.email, 'reset', ip);
  }

  private validatePasswords(password: string, confirmPassword: string) {
    if (password !== confirmPassword) throw new BadRequestException('两次输入的密码不一致');
  }

  private findByLoginCredential(value: string) {
    const account = this.normalizeLoginCredential(value);
    const emailLike = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(account);
    return this.prisma.user.findFirst({
      where: emailLike
        ? { email: account }
        : {
          OR: [
            { phone: account },
            // 兼容尚未绑定邮箱或手机号的历史账号；绑定后用户名不再可用于登录。
            { AND: [{ email: null }, { phone: null }, { username: account }] },
          ],
        },
    });
  }

  private normalizeLoginCredential(value: string) {
    const account = value.trim();
    return account.includes('@') ? account.toLowerCase() : account;
  }

  private normalizeEmail(value: string) {
    const email = value.trim().toLowerCase();
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) throw new BadRequestException('请输入正确的邮箱地址');
    return email;
  }

  private normalizePhone(value: string) {
    const phone = value.trim().replace(/[\s-]/g, '');
    if (!/^\+?\d{6,20}$/.test(phone)) throw new BadRequestException('请输入正确的手机号');
    return phone;
  }

  private async assertContactAvailable(type: 'email' | 'phone', value: string, excludeId: string) {
    const exists = await this.prisma.user.findFirst({
      where: { id: { not: excludeId }, [type]: value },
    });
    if (exists) throw new BadRequestException(type === 'email' ? '该邮箱已被其他账号使用' : '该手机号已被其他账号使用');
  }

  async refresh(user: { sub: string; sid: string; uv: number; username: string; role: string; tenantId: string | null; portal?: AuthPortal }) {
    const portal = user.portal ?? 'tenant';
    const session = await this.sessions.touch(user.sid, user.sub, portal);
    return {
      accessToken: this.jwtService.sign({ sub: user.sub, sid: user.sid, uv: user.uv, username: user.username, role: user.role, tenantId: user.tenantId, portal }),
      idleTimeoutSeconds: session.idleTimeoutSeconds,
      absoluteExpiresAt: session.absoluteExpiresAt,
    };
  }

  async logout(user: { sid?: string }) {
    await this.sessions.revoke(user.sid);
    return { message: '已退出登录' };
  }

  private async signToken(user: User, portal: AuthPortal = 'tenant') {
    const created = await this.sessions.create(user.id, portal);
    const accessToken = this.jwtService.sign({
      sub: user.id,
      sid: created.session.id,
      uv: user.updatedAt.getTime(),
      username: user.username,
      role: user.role,
      tenantId: user.tenantId,
      portal,
    });
    return { accessToken, idleTimeoutSeconds: created.idleTimeoutSeconds, absoluteExpiresAt: created.session.absoluteExpiresAt };
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

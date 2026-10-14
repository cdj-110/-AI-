import { Body, Controller, Get, Param, Patch, Post, Req, Res, UploadedFile, UseGuards, UseInterceptors } from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { Request, Response } from 'express';
import { CurrentUser, AuthUser } from '../common/decorators/current-user.decorator';
import { RawResponse } from '../common/decorators/raw-response.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { AuthService } from './auth.service';
import { LoginDto } from './dto/login.dto';
import { RegisterDto } from './dto/register.dto';
import { ResetPasswordDto } from './dto/reset-password.dto';
import { VerifyCaptchaDto } from './dto/captcha.dto';
import { CaptchaService } from './captcha.service';
import { SendEmailCodeDto } from './dto/send-email-code.dto';
import { AccountContactCodeDto, UpdateAccountContactDto } from './dto/account-contact.dto';
import { UpdateProfileDto } from './dto/update-profile.dto';
import { LoginWithCodeDto, SendLoginCodeDto } from './dto/login-code.dto';

@Controller('auth')
export class AuthController {
  constructor(
    private readonly authService: AuthService,
    private readonly captchaService: CaptchaService,
  ) {}

  @Get('captcha')
  captcha(@Req() request: Request) {
    return this.captchaService.create(this.clientIp(request));
  }

  @Post('captcha/verify')
  verifyCaptcha(@Body() dto: VerifyCaptchaDto, @Req() request: Request) {
    return this.captchaService.verify(dto.challengeId, dto.offsetX, dto.trail, this.clientIp(request));
  }

  @Post('login')
  login(@Body() dto: LoginDto, @Req() request: Request) {
    return this.authService.login(dto, {
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    });
  }

  @Post('login/admin')
  loginAdmin(@Body() dto: LoginDto, @Req() request: Request) {
    return this.authService.login(dto, {
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    }, 'admin');
  }

  @Post('login/tenant')
  loginTenant(@Body() dto: LoginDto, @Req() request: Request) {
    return this.authService.login(dto, {
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    }, 'tenant');
  }

  @Post('login/admin/code/send')
  sendAdminLoginCode(@Body() dto: SendLoginCodeDto, @Req() request: Request) {
    return this.authService.sendLoginCode(dto, this.clientIp(request), 'admin');
  }

  @Post('login/admin/code')
  loginAdminWithCode(@Body() dto: LoginWithCodeDto, @Req() request: Request) {
    return this.authService.loginWithCode(dto, {
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    }, 'admin');
  }

  @Post('login/tenant/code/send')
  sendTenantLoginCode(@Body() dto: SendLoginCodeDto, @Req() request: Request) {
    return this.authService.sendLoginCode(dto, this.clientIp(request), 'tenant');
  }

  @Post('login/tenant/code')
  loginTenantWithCode(@Body() dto: LoginWithCodeDto, @Req() request: Request) {
    return this.authService.loginWithCode(dto, {
      ip: this.clientIp(request),
      userAgent: request.headers['user-agent'],
    }, 'tenant');
  }

  @Get('profile')
  @UseGuards(JwtAuthGuard)
  profile(@CurrentUser() user: AuthUser) {
    return this.authService.profile(user.sub);
  }

  @Get('avatars/:userId/:filename')
  @RawResponse()
  async avatar(@Param('userId') userId: string, @Res() response: Response) {
    const file = await this.authService.avatarFile(userId);
    response.setHeader('Cache-Control', 'public, max-age=31536000, immutable');
    response.type(file.mimeType);
    return response.sendFile(file.absolutePath);
  }

  @Patch('account/profile')
  @UseGuards(JwtAuthGuard)
  updateProfile(@CurrentUser() user: AuthUser, @Body() dto: UpdateProfileDto) {
    return this.authService.updateProfile(user, dto);
  }

  @Post('account/avatar')
  @UseGuards(JwtAuthGuard)
  @UseInterceptors(FileInterceptor('avatar', { limits: { fileSize: 2 * 1024 * 1024, files: 1 } }))
  uploadAvatar(
    @CurrentUser() user: AuthUser,
    @UploadedFile() file?: { buffer: Buffer; mimetype: string; size: number },
  ) {
    return this.authService.updateAvatar(user, file);
  }

  @Post('account/contact-code')
  @UseGuards(JwtAuthGuard)
  sendAccountContactCode(@CurrentUser() user: AuthUser, @Body() dto: AccountContactCodeDto, @Req() request: Request) {
    return this.authService.sendAccountContactCode(user.sub, dto, this.clientIp(request));
  }

  @Patch('account/contact')
  @UseGuards(JwtAuthGuard)
  updateAccountContact(@CurrentUser() user: AuthUser, @Body() dto: UpdateAccountContactDto) {
    return this.authService.updateAccountContact(user, dto);
  }

  @Post('activity')
  @UseGuards(JwtAuthGuard)
  activity(@CurrentUser() user: AuthUser) {
    return this.authService.refresh(user);
  }

  @Post('logout')
  @UseGuards(JwtAuthGuard)
  logout(@CurrentUser() user: AuthUser) {
    return this.authService.logout(user);
  }

  @Post('register')
  register(@Body() dto: RegisterDto) {
    return this.authService.register(dto);
  }

  @Post('email-code')
  sendEmailCode(@Body() dto: SendEmailCodeDto, @Req() request: Request) {
    return this.authService.sendEmailCode(dto, this.clientIp(request));
  }

  @Post('forgot-password/reset')
  resetPassword(@Body() dto: ResetPasswordDto) {
    return this.authService.resetPassword(dto);
  }

  private clientIp(request: Request) {
    const forwardedFor = request.headers['x-forwarded-for'];
    const firstForwardedIp = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor?.split(',')[0];
    return (firstForwardedIp || request.headers['x-real-ip'] || request.ip || request.socket.remoteAddress || '').toString().replace('::ffff:', '');
  }
}

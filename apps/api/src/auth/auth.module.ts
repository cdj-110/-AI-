import { Module } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtModule } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import { AuthController } from './auth.controller';
import { AuthService } from './auth.service';
import { JwtStrategy } from './jwt.strategy';
import { AuthSessionService } from './auth-session.service';
import { CaptchaService } from './captcha.service';
import { EmailVerificationService } from './email-verification.service';

@Module({
  imports: [
    PassportModule,
    JwtModule.registerAsync({
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        secret: config.get<string>('JWT_SECRET', 'weikong_iot_secret'),
        signOptions: { expiresIn: config.get<string>('JWT_EXPIRES_IN', '30m') },
      }),
    }),
  ],
  controllers: [AuthController],
  providers: [AuthService, JwtStrategy, AuthSessionService, CaptchaService, EmailVerificationService],
  exports: [JwtModule, JwtStrategy, AuthSessionService],
})
export class AuthModule {}

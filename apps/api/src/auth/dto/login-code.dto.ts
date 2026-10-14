import { IsNotEmpty, IsString, Matches } from 'class-validator';

export class SendLoginCodeDto {
  @IsString()
  @IsNotEmpty()
  username: string;

  @IsString()
  @IsNotEmpty()
  captchaToken: string;
}

export class LoginWithCodeDto {
  @IsString()
  @IsNotEmpty()
  username: string;

  @IsString()
  @Matches(/^\d{6}$/, { message: '验证码必须为 6 位数字' })
  code: string;
}

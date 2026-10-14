import { IsEmail, IsNotEmpty, IsString, Matches, MinLength } from 'class-validator';

export class RegisterDto {
  @IsString()
  @IsNotEmpty()
  username: string;

  @IsEmail()
  email: string;

  @IsString()
  @Matches(/^\d{6}$/, { message: '验证码必须为 6 位数字' })
  code: string;

  @IsString()
  @MinLength(6)
  password: string;

  @IsString()
  confirmPassword: string;
}

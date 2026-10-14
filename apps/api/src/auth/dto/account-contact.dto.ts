import { IsIn, IsNotEmpty, IsString, Matches } from 'class-validator';

export class AccountContactCodeDto {
  @IsString()
  @IsIn(['email', 'phone'])
  type: 'email' | 'phone';

  @IsString()
  @IsNotEmpty()
  value: string;
}

export class UpdateAccountContactDto extends AccountContactCodeDto {
  @IsString()
  @Matches(/^\d{6}$/, { message: '验证码必须为 6 位数字' })
  code: string;
}

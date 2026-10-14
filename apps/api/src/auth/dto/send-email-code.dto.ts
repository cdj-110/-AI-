import { IsIn, IsNotEmpty, IsString } from 'class-validator';

export class SendEmailCodeDto {
  @IsString()
  @IsIn(['register', 'reset'])
  purpose: 'register' | 'reset';

  @IsString()
  @IsNotEmpty()
  target: string;
}

import { IsNotEmpty, IsString, MinLength } from 'class-validator';

export class ResetPasswordDto {
  @IsString()
  @IsNotEmpty()
  username: string;

  @IsString()
  code: string;

  @IsString()
  @MinLength(6)
  password: string;

  @IsString()
  confirmPassword: string;
}

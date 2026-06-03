import { IsEmail, IsIn, IsNotEmpty, IsOptional, IsString, MinLength } from 'class-validator';

export class CreateUserDto {
  @IsString()
  @IsNotEmpty()
  username: string;

  @IsString()
  @MinLength(6)
  password: string;

  @IsString()
  @IsOptional()
  nickname?: string;

  @IsString()
  @IsOptional()
  phone?: string;

  @IsEmail()
  @IsOptional()
  email?: string;

  @IsString()
  @IsOptional()
  tenantId?: string;

  @IsIn(['SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER'])
  role: string;

  @IsIn(['ACTIVE', 'DISABLED'])
  @IsOptional()
  status?: string;
}

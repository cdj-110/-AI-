import { ArrayMaxSize, ArrayUnique, IsArray, IsEmail, IsIn, IsNotEmpty, IsOptional, IsString, IsUUID, MaxLength, MinLength, ValidateIf } from 'class-validator';
import { TENANT_USER_PERMISSIONS } from '../../common/user-permissions';

export class CreateUserDto {
  @IsString()
  @IsNotEmpty()
  @MaxLength(50, { message: '用户名称不能超过 50 个字符' })
  username: string;

  @IsString()
  @MinLength(6)
  password: string;

  @IsString()
  @IsOptional()
  nickname?: string;

  @IsString()
  @IsOptional()
  @ValidateIf((_object, value) => value !== '')
  phone?: string;

  @IsEmail()
  @IsOptional()
  @ValidateIf((_object, value) => value !== '')
  email?: string;

  @IsString()
  @IsOptional()
  tenantId?: string;

  @IsIn(['SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER'])
  role: string;

  @IsIn(['ACTIVE', 'DISABLED'])
  @IsOptional()
  status?: string;

  @IsArray()
  @ArrayUnique()
  @IsIn(TENANT_USER_PERMISSIONS, { each: true })
  @IsOptional()
  permissions?: string[];

  @IsIn(['ALL', 'CUSTOM'])
  @IsOptional()
  deviceScopeMode?: 'ALL' | 'CUSTOM';

  @IsArray()
  @ArrayUnique()
  @ArrayMaxSize(2000)
  @IsUUID('4', { each: true })
  @IsOptional()
  deviceIds?: string[];

  @IsArray()
  @ArrayUnique()
  @ArrayMaxSize(500)
  @IsUUID('4', { each: true })
  @IsOptional()
  groupIds?: string[];
}

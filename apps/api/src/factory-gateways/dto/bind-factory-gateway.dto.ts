import { IsNotEmpty, IsOptional, IsString, Matches, MaxLength } from 'class-validator';

export class BindFactoryGatewayDto {
  @IsString()
  @IsNotEmpty()
  @MaxLength(32)
  sn: string;

  @IsString()
  @IsOptional()
  @Matches(/^\d{6}$/, { message: '绑定码必须是 6 位数字' })
  bindCode?: string;

  @IsString()
  @IsOptional()
  @MaxLength(128)
  nfcToken?: string;

  @IsString()
  @IsOptional()
  @MaxLength(64)
  name?: string;

  @IsString()
  @IsOptional()
  @MaxLength(128)
  location?: string;
}

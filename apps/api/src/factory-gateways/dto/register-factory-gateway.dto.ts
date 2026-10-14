import { IsNotEmpty, IsOptional, IsString, MaxLength } from 'class-validator';

export class RegisterFactoryGatewayDto {
  @IsString()
  @IsNotEmpty()
  hardwareId: string;

  @IsString()
  @IsNotEmpty()
  chipUsid: string;

  @IsString()
  @IsOptional()
  @MaxLength(64)
  batchNo?: string;

  @IsString()
  @IsOptional()
  @MaxLength(32)
  model?: string;
}

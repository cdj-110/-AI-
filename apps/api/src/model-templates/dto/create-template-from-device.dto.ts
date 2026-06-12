import { IsOptional, IsString, IsUUID } from 'class-validator';

export class CreateTemplateFromDeviceDto {
  @IsUUID()
  deviceId: string;

  @IsString()
  @IsOptional()
  name?: string;

  @IsString()
  @IsOptional()
  description?: string;
}

import { Type } from 'class-transformer';
import { IsIn, IsNotEmpty, IsNumber, IsOptional, IsString, IsUUID, Max, Min } from 'class-validator';

export class CreateDeviceDto {
  @IsString()
  @IsNotEmpty()
  deviceKey: string;

  @IsString()
  @IsNotEmpty()
  name: string;

  @IsIn(['MQTT', 'HTTP', 'MODBUS'])
  @IsOptional()
  protocol?: string;

  @IsIn(['GATEWAY', 'GATEWAY_CHILD', 'DIRECT'])
  @IsOptional()
  deviceType?: string;

  @IsUUID()
  @IsOptional()
  gatewayId?: string;

  @IsString()
  @IsOptional()
  location?: string;

  @Type(() => Number)
  @IsNumber()
  @Min(-90)
  @Max(90)
  @IsOptional()
  latitude?: number;

  @Type(() => Number)
  @IsNumber()
  @Min(-180)
  @Max(180)
  @IsOptional()
  longitude?: number;

  @IsUUID()
  @IsOptional()
  templateDeviceId?: string;

  @IsUUID()
  @IsOptional()
  modelTemplateId?: string;
}

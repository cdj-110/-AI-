import { IsIn, IsNotEmpty, IsOptional, IsString, IsUUID } from 'class-validator';

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

  @IsString()
  @IsOptional()
  location?: string;

  @IsUUID()
  @IsOptional()
  templateDeviceId?: string;
}

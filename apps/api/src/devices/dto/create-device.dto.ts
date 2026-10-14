import { Type } from 'class-transformer';
import { IsIn, IsNotEmpty, IsNumber, IsOptional, IsString, IsUUID, Max, Min } from 'class-validator';

// Keep this list aligned with the protocols reported by the edge gateway.  Cloud
// device records describe both direct connections and gateway child devices, so
// restricting this field to the three cloud ingress protocols rejects valid
// industrial child devices during an otherwise unrelated edit.
export const DEVICE_PROTOCOLS = [
  'MQTT',
  'HTTP',
  'MODBUS',
  'modbus-tcp',
  'modbus-rtu',
  'dlt645',
  'iec104',
  'iec61850',
  'iec61850-goose',
  'siemens-s7',
  'opcua',
  'opcda',
];

export class CreateDeviceDto {
  @IsString()
  @IsNotEmpty()
  deviceKey: string;

  @IsString()
  @IsNotEmpty()
  name: string;

  @IsIn(DEVICE_PROTOCOLS, { message: '请选择系统支持的接入协议' })
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

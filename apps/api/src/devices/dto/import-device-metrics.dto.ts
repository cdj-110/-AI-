import { IsBoolean, IsOptional, IsUUID } from 'class-validator';

export class ImportDeviceMetricsDto {
  @IsUUID()
  templateDeviceId: string;

  @IsBoolean()
  @IsOptional()
  overwrite?: boolean;
}

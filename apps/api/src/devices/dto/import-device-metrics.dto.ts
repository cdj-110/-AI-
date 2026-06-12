import { IsBoolean, IsOptional, IsUUID } from 'class-validator';

export class ImportDeviceMetricsDto {
  @IsUUID()
  @IsOptional()
  templateDeviceId?: string;

  @IsUUID()
  @IsOptional()
  modelTemplateId?: string;

  @IsBoolean()
  @IsOptional()
  overwrite?: boolean;
}

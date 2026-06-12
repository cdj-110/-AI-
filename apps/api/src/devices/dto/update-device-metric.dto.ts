import { IsBoolean, IsIn, IsInt, IsOptional, IsString, Max, Min } from 'class-validator';

export class UpdateDeviceMetricDto {
  @IsString()
  @IsOptional()
  name?: string;

  @IsString()
  @IsOptional()
  unit?: string;

  @IsIn(['NUMBER', 'STRING', 'BOOLEAN', 'OBJECT'])
  @IsOptional()
  dataType?: string;

  @IsInt()
  @Min(0)
  @Max(6)
  @IsOptional()
  decimals?: number;

  @IsIn(['READ_ONLY', 'READ_WRITE'])
  @IsOptional()
  accessMode?: string;

  @IsBoolean()
  @IsOptional()
  enabled?: boolean;

  @IsBoolean()
  @IsOptional()
  ignored?: boolean;

  @IsInt()
  @Min(0)
  @IsOptional()
  sortOrder?: number;
}

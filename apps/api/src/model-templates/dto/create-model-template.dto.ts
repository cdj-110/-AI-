import { Type } from 'class-transformer';
import { IsArray, IsBoolean, IsIn, IsInt, IsNotEmpty, IsOptional, IsString, ValidateNested } from 'class-validator';

export class ModelTemplateMetricDto {
  @IsString()
  @IsNotEmpty()
  identifier: string;

  @IsString()
  @IsNotEmpty()
  name: string;

  @IsIn(['NUMBER', 'STRING', 'BOOLEAN', 'OBJECT'])
  dataType: string;

  @IsString()
  @IsOptional()
  unit?: string;

  @IsInt()
  @IsOptional()
  decimals?: number;

  @IsIn(['READ_ONLY', 'READ_WRITE'])
  @IsOptional()
  accessMode?: string;

  @IsBoolean()
  @IsOptional()
  enabled?: boolean;

  @IsInt()
  @IsOptional()
  sortOrder?: number;
}

export class CreateModelTemplateDto {
  @IsString()
  @IsNotEmpty()
  name: string;

  @IsString()
  @IsOptional()
  description?: string;

  @IsIn(['GATEWAY', 'GATEWAY_CHILD', 'DIRECT'])
  @IsOptional()
  deviceType?: string;

  @IsArray()
  @ValidateNested({ each: true })
  @Type(() => ModelTemplateMetricDto)
  @IsOptional()
  metrics?: ModelTemplateMetricDto[];
}

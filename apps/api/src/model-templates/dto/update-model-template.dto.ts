import { PartialType } from '@nestjs/mapped-types';
import { Type } from 'class-transformer';
import { IsArray, ValidateNested } from 'class-validator';
import { CreateModelTemplateDto, ModelTemplateMetricDto } from './create-model-template.dto';

export class UpdateModelTemplateDto extends PartialType(CreateModelTemplateDto) {}

export class SaveModelTemplateMetricsDto {
  @IsArray()
  @ValidateNested({ each: true })
  @Type(() => ModelTemplateMetricDto)
  metrics: ModelTemplateMetricDto[];
}

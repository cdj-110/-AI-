import { PartialType } from '@nestjs/mapped-types';
import { Type } from 'class-transformer';
import { ArrayMaxSize, ArrayMinSize, IsArray, IsDateString, IsIn, IsOptional, IsString, MaxLength, ValidateNested } from 'class-validator';

const SURFACES = ['ADMIN', 'TENANT', 'GATEWAY'] as const;
const CATEGORIES = ['FEATURE', 'IMPROVEMENT', 'FIX', 'SECURITY'] as const;
const STATUSES = ['DRAFT', 'PUBLISHED'] as const;

export class ChangelogItemDto {
  @IsString()
  @MaxLength(300)
  text!: string;
}

export class CreateChangelogEntryDto {
  @IsOptional()
  @IsString()
  @MaxLength(30)
  version?: string;

  @IsString()
  @MaxLength(120)
  title!: string;

  @IsOptional()
  @IsString()
  @MaxLength(300)
  summary?: string;

  @IsDateString()
  releaseDate!: string;

  @IsArray()
  @ArrayMinSize(1)
  @ArrayMaxSize(3)
  @IsIn(SURFACES, { each: true })
  surfaces!: string[];

  @IsIn(CATEGORIES)
  category!: string;

  @IsArray()
  @ArrayMinSize(1)
  @ArrayMaxSize(30)
  @ValidateNested({ each: true })
  @Type(() => ChangelogItemDto)
  items!: ChangelogItemDto[];

  @IsOptional()
  @IsIn(STATUSES)
  status?: string;
}

export class UpdateChangelogEntryDto extends PartialType(CreateChangelogEntryDto) {}


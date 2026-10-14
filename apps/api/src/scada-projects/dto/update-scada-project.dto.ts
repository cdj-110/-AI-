import { IsIn, IsObject, IsOptional, IsString, MaxLength, MinLength } from 'class-validator';

export class UpdateScadaProjectDto {
  @IsString()
  @MinLength(1)
  @MaxLength(80)
  @IsOptional()
  name?: string;

  @IsString()
  @MaxLength(300)
  @IsOptional()
  description?: string;

  @IsIn(['DRAFT', 'PUBLISHED'])
  @IsOptional()
  status?: string;

  @IsString()
  @IsOptional()
  thumbnail?: string;

  @IsObject()
  @IsOptional()
  config?: Record<string, unknown>;
}

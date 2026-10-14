import { IsOptional, IsString, MaxLength, MinLength } from 'class-validator';

export class CreateScadaProjectDto {
  @IsString()
  @MinLength(1)
  @MaxLength(80)
  name!: string;

  @IsString()
  @IsOptional()
  @MaxLength(300)
  description?: string;
}

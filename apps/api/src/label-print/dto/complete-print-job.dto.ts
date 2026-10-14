import { IsBoolean, IsOptional, IsString, MaxLength } from 'class-validator';

export class CompletePrintJobDto {
  @IsBoolean()
  success!: boolean;

  @IsOptional()
  @IsString()
  @MaxLength(500)
  error?: string;

  @IsOptional()
  @IsString()
  @MaxLength(128)
  printerName?: string;
}

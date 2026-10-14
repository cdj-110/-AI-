import { IsOptional, IsString, MaxLength } from 'class-validator';

export class CreatePrintAgentDto {
  @IsString()
  @MaxLength(64)
  name!: string;

  @IsOptional()
  @IsString()
  @MaxLength(128)
  printerName?: string;
}

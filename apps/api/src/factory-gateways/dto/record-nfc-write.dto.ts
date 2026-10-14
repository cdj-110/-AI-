import { IsBoolean, IsOptional, IsString, MaxLength } from 'class-validator';

export class RecordNfcWriteDto {
  @IsString()
  @MaxLength(64)
  sn: string;

  @IsBoolean()
  success: boolean;

  @IsString()
  @IsOptional()
  @MaxLength(500)
  message?: string;
}

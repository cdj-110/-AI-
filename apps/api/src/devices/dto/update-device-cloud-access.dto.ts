import { IsBoolean, IsOptional, IsString, Length } from 'class-validator';

export class UpdateDeviceCloudAccessDto {
  @IsBoolean()
  enabled!: boolean;

  @IsOptional()
  @IsString()
  @Length(2, 200)
  reason?: string;
}

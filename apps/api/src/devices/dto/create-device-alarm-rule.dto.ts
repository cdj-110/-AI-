import { IsBoolean, IsIn, IsNumber, IsOptional, IsString, Min } from 'class-validator';

export class CreateDeviceAlarmRuleDto {
  @IsString()
  identifier: string;

  @IsIn(['>', '>=', '<', '<='])
  operator: string;

  @IsNumber()
  threshold: number;

  @IsNumber()
  @Min(0)
  @IsOptional()
  hysteresis?: number;

  @IsIn(['INFO', 'WARNING', 'CRITICAL'])
  @IsOptional()
  level?: string;

  @IsBoolean()
  @IsOptional()
  enabled?: boolean;
}

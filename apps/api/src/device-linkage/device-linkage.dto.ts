import { IsBoolean, IsDefined, IsIn, IsNumber, IsString, IsUUID, MaxLength, MinLength } from 'class-validator';

export class SaveDeviceLinkageDto {
  @IsString() @MinLength(1) @MaxLength(80) name: string;
  @IsUUID() sourceDeviceId: string;
  @IsString() @MinLength(1) @MaxLength(128) sourceMetric: string;
  @IsIn(['GT', 'GTE', 'LT', 'LTE', 'EQ', 'NE']) operator: string;
  @IsNumber({ allowNaN: false, allowInfinity: false }) threshold: number;
  @IsUUID() targetDeviceId: string;
  @IsString() @MinLength(1) @MaxLength(128) targetMetric: string;
  @IsDefined() actionValue: unknown;
}
export class EnableDeviceLinkageDto {
  @IsBoolean() enabled: boolean;
  @IsBoolean() confirmAutomaticControl: boolean;
}

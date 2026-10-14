import { IsDefined, IsString, IsUUID, MaxLength, MinLength } from 'class-validator';

export class CreateDeviceCommandDto {
  @IsUUID('4')
  requestId: string;

  @IsString()
  @MinLength(1)
  @MaxLength(128)
  metric: string;

  @IsDefined()
  value: unknown;
}

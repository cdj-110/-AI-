import { IsNotEmpty, IsOptional, IsString } from 'class-validator';

export class RegisterFactoryGatewayDto {
  @IsString()
  @IsNotEmpty()
  hardwareId: string;

  @IsString()
  @IsOptional()
  batchNo?: string;
}

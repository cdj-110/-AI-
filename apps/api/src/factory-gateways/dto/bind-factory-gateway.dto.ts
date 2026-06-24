import { IsNotEmpty, IsOptional, IsString } from 'class-validator';

export class BindFactoryGatewayDto {
  @IsString()
  @IsNotEmpty()
  sn: string;

  @IsString()
  @IsNotEmpty()
  bindCode: string;

  @IsString()
  @IsOptional()
  name?: string;

  @IsString()
  @IsOptional()
  location?: string;
}

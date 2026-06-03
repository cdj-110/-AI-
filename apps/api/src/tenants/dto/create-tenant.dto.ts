import { IsIn, IsNotEmpty, IsOptional, IsString } from 'class-validator';

export class CreateTenantDto {
  @IsString()
  @IsNotEmpty()
  name: string;

  @IsIn(['ACTIVE', 'DISABLED'])
  @IsOptional()
  status?: string;
}

import { IsIn, IsInt, IsString, Max, Min } from 'class-validator';

export class CreatePrintJobDto {
  @IsString()
  agentId!: string;

  @IsString()
  factoryGatewayId!: string;

  @IsIn(['50x30', '60x40', '70x50'])
  labelSize!: string;

  @IsInt()
  @Min(1)
  @Max(20)
  copies!: number;
}

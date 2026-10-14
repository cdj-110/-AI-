import { IsIn } from 'class-validator';

export class CreateMaintenanceTaskDto {
  @IsIn(['SERVICE_RESTART', 'GATEWAY_REBOOT'])
  action: 'SERVICE_RESTART' | 'GATEWAY_REBOOT';
}

import { IsIn } from 'class-validator';

export class ReportDeviceStatusDto {
  @IsIn(['ONLINE', 'OFFLINE'])
  status: string;
}

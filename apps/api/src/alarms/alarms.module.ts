import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { AlarmsController } from './alarms.controller';
import { AlarmsService } from './alarms.service';

@Module({
  imports: [OperationLogsModule],
  controllers: [AlarmsController],
  providers: [AlarmsService],
})
export class AlarmsModule {}

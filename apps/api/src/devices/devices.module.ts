import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { DevicesController } from './devices.controller';
import { DevicesService } from './devices.service';

@Module({
  imports: [OperationLogsModule],
  controllers: [DevicesController],
  providers: [DevicesService],
})
export class DevicesModule {}

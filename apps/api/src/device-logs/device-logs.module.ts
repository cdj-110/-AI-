import { Module } from '@nestjs/common';
import { DeviceLogsController } from './device-logs.controller';
import { DeviceLogsService } from './device-logs.service';

@Module({
  controllers: [DeviceLogsController],
  providers: [DeviceLogsService],
})
export class DeviceLogsModule {}

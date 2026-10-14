import { Module } from '@nestjs/common';
import { AuthModule } from '../auth/auth.module';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { MqttAuthModule } from '../mqtt/mqtt-auth.module';
import { RemoteAccessModule } from '../remote-access/remote-access.module';
import { DevicesController } from './devices.controller';
import { DevicesRealtimeGateway } from './devices-realtime.gateway';
import { DevicesService } from './devices.service';
import { DeviceGroupsController } from './device-groups.controller';
import { DeviceGroupsService } from './device-groups.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { HistoryDataTasksController } from './history-data-tasks.controller';
import { HistoryDataTasksService } from './history-data-tasks.service';

@Module({
  imports: [AuthModule, OperationLogsModule, MqttAuthModule, RemoteAccessModule, SubscriptionsModule],
  controllers: [DevicesController, DeviceGroupsController, HistoryDataTasksController],
  providers: [DevicesService, DevicesRealtimeGateway, DeviceGroupsService, HistoryDataTasksService],
  exports: [DevicesService],
})
export class DevicesModule {}

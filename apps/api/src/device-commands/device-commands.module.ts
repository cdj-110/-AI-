import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { DeviceCommandsController } from './device-commands.controller';
import { DeviceCommandsService } from './device-commands.service';
import { DeviceCommandsMqttService } from './device-commands-mqtt.service';

@Module({ imports: [OperationLogsModule], controllers: [DeviceCommandsController], providers: [DeviceCommandsService, DeviceCommandsMqttService], exports: [DeviceCommandsService] })
export class DeviceCommandsModule {}

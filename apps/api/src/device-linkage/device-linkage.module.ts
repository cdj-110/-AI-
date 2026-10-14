import { Module } from '@nestjs/common';
import { DevicesModule } from '../devices/devices.module';
import { DeviceCommandsModule } from '../device-commands/device-commands.module';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { DeviceLinkageController } from './device-linkage.controller';
import { DeviceLinkageService } from './device-linkage.service';
@Module({ imports: [DevicesModule, DeviceCommandsModule, OperationLogsModule], controllers: [DeviceLinkageController], providers: [DeviceLinkageService] })
export class DeviceLinkageModule {}

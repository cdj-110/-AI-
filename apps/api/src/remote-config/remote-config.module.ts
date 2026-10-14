import { Module } from '@nestjs/common';
import { RemoteConfigController } from './remote-config.controller';
import { RemoteConfigMqttService } from './remote-config-mqtt.service';
import { RemoteConfigService } from './remote-config.service';

@Module({
  controllers: [RemoteConfigController],
  providers: [RemoteConfigService, RemoteConfigMqttService],
})
export class RemoteConfigModule {}

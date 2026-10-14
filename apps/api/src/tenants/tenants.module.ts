import { Module } from '@nestjs/common';
import { TenantsController } from './tenants.controller';
import { TenantsService } from './tenants.service';
import { MqttAuthModule } from '../mqtt/mqtt-auth.module';
import { RemoteAccessModule } from '../remote-access/remote-access.module';

@Module({
  imports: [MqttAuthModule, RemoteAccessModule],
  controllers: [TenantsController],
  providers: [TenantsService],
})
export class TenantsModule {}

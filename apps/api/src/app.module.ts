import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { AlarmsModule } from './alarms/alarms.module';
import { AuthModule } from './auth/auth.module';
import { DashboardModule } from './dashboard/dashboard.module';
import { DeviceLogsModule } from './device-logs/device-logs.module';
import { DevicesModule } from './devices/devices.module';
import { FactoryGatewaysModule } from './factory-gateways/factory-gateways.module';
import { LoginLogsModule } from './login-logs/login-logs.module';
import { ModelTemplatesModule } from './model-templates/model-templates.module';
import { MqttAuthModule } from './mqtt/mqtt-auth.module';
import { OperationLogsModule } from './operation-logs/operation-logs.module';
import { RemoteConfigModule } from './remote-config/remote-config.module';
import { PrismaModule } from './prisma/prisma.module';
import { SystemStatusModule } from './system-status/system-status.module';
import { TenantsModule } from './tenants/tenants.module';
import { UsersModule } from './users/users.module';

@Module({
  imports: [
    ConfigModule.forRoot({ isGlobal: true }),
    PrismaModule,
    AuthModule,
    UsersModule,
    TenantsModule,
    DashboardModule,
    DeviceLogsModule,
    DevicesModule,
    FactoryGatewaysModule,
    LoginLogsModule,
    ModelTemplatesModule,
    MqttAuthModule,
    OperationLogsModule,
    RemoteConfigModule,
    SystemStatusModule,
    AlarmsModule,
  ],
})
export class AppModule {}

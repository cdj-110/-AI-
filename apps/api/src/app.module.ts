import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { AlarmsModule } from './alarms/alarms.module';
import { AuthModule } from './auth/auth.module';
import { DashboardModule } from './dashboard/dashboard.module';
import { DeviceLogsModule } from './device-logs/device-logs.module';
import { DevicesModule } from './devices/devices.module';
import { DeviceCommandsModule } from './device-commands/device-commands.module';
import { FactoryGatewaysModule } from './factory-gateways/factory-gateways.module';
import { EventsModule } from './events/events.module';
import { LabelPrintModule } from './label-print/label-print.module';
import { GatewayTopologyModule } from './gateway-topology/gateway-topology.module';
import { LoginLogsModule } from './login-logs/login-logs.module';
import { ModelTemplatesModule } from './model-templates/model-templates.module';
import { MqttAuthModule } from './mqtt/mqtt-auth.module';
import { OperationLogsModule } from './operation-logs/operation-logs.module';
import { RemoteConfigModule } from './remote-config/remote-config.module';
import { RemoteAccessModule } from './remote-access/remote-access.module';
import { PrismaModule } from './prisma/prisma.module';
import { SystemStatusModule } from './system-status/system-status.module';
import { TenantsModule } from './tenants/tenants.module';
import { UsersModule } from './users/users.module';
import { ProductsModule } from './products/products.module';
import { OtaModule } from './ota/ota.module';
import { GatewayMaintenanceModule } from './gateway-maintenance/gateway-maintenance.module';
import { ScadaProjectsModule } from './scada-projects/scada-projects.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { ChangelogModule } from './changelog/changelog.module';
import { DeviceLinkageModule } from './device-linkage/device-linkage.module';

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
    DeviceCommandsModule,
    EventsModule,
    FactoryGatewaysModule,
    ProductsModule,
    OtaModule,
    GatewayMaintenanceModule,
    LabelPrintModule,
    GatewayTopologyModule,
    LoginLogsModule,
    ModelTemplatesModule,
    MqttAuthModule,
    OperationLogsModule,
    RemoteConfigModule,
    RemoteAccessModule,
    SystemStatusModule,
    AlarmsModule,
    ScadaProjectsModule,
    SubscriptionsModule,
    ChangelogModule,
    DeviceLinkageModule,
  ],
})
export class AppModule {}

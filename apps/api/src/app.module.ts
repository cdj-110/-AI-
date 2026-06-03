import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { AlarmsModule } from './alarms/alarms.module';
import { AuthModule } from './auth/auth.module';
import { DashboardModule } from './dashboard/dashboard.module';
import { DevicesModule } from './devices/devices.module';
import { PrismaModule } from './prisma/prisma.module';
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
    DevicesModule,
    AlarmsModule,
  ],
})
export class AppModule {}

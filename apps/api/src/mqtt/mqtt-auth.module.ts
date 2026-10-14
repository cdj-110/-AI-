import { Module } from '@nestjs/common';
import { PrismaModule } from '../prisma/prisma.module';
import { MqttAuthController } from './mqtt-auth.controller';
import { MqttConnectionAdminService } from './mqtt-connection-admin.service';

@Module({
  imports: [PrismaModule],
  controllers: [MqttAuthController],
  providers: [MqttConnectionAdminService],
  exports: [MqttConnectionAdminService],
})
export class MqttAuthModule {}

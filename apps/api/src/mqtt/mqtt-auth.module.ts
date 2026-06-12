import { Module } from '@nestjs/common';
import { PrismaModule } from '../prisma/prisma.module';
import { MqttAuthController } from './mqtt-auth.controller';

@Module({
  imports: [PrismaModule],
  controllers: [MqttAuthController],
})
export class MqttAuthModule {}

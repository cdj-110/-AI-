import { Module } from '@nestjs/common';
import { ScadaProjectsController } from './scada-projects.controller';
import { ScadaProjectsService } from './scada-projects.service';
import { GoViewCompatController } from './goview-compat.controller';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { DevicesModule } from '../devices/devices.module';

@Module({
  imports: [SubscriptionsModule, DevicesModule],
  controllers: [ScadaProjectsController, GoViewCompatController],
  providers: [ScadaProjectsService],
})
export class ScadaProjectsModule {}

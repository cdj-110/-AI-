import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { GatewayTopologyController } from './gateway-topology.controller';
import { GatewayTopologyService } from './gateway-topology.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [OperationLogsModule, SubscriptionsModule],
  controllers: [GatewayTopologyController],
  providers: [GatewayTopologyService],
  exports: [GatewayTopologyService],
})
export class GatewayTopologyModule {}

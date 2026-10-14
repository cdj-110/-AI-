import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { FactoryGatewaysController } from './factory-gateways.controller';
import { FactoryGatewaysService } from './factory-gateways.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [OperationLogsModule, SubscriptionsModule],
  controllers: [FactoryGatewaysController],
  providers: [FactoryGatewaysService],
  exports: [FactoryGatewaysService],
})
export class FactoryGatewaysModule {}

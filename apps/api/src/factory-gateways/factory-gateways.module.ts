import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { FactoryGatewaysController } from './factory-gateways.controller';
import { FactoryGatewaysService } from './factory-gateways.service';

@Module({
  imports: [OperationLogsModule],
  controllers: [FactoryGatewaysController],
  providers: [FactoryGatewaysService],
  exports: [FactoryGatewaysService],
})
export class FactoryGatewaysModule {}

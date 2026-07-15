import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { GatewayTopologyController } from './gateway-topology.controller';
import { GatewayTopologyService } from './gateway-topology.service';

@Module({
  imports: [OperationLogsModule],
  controllers: [GatewayTopologyController],
  providers: [GatewayTopologyService],
  exports: [GatewayTopologyService],
})
export class GatewayTopologyModule {}

import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { GatewayMaintenanceController } from './gateway-maintenance.controller';
import { GatewayMaintenanceService } from './gateway-maintenance.service';

@Module({ imports: [OperationLogsModule], controllers: [GatewayMaintenanceController], providers: [GatewayMaintenanceService] })
export class GatewayMaintenanceModule {}

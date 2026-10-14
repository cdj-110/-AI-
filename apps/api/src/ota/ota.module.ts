import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { OtaController } from './ota.controller';
import { OtaService } from './ota.service';

@Module({ imports: [OperationLogsModule], controllers: [OtaController], providers: [OtaService] })
export class OtaModule {}

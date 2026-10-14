import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { RemoteAccessController } from './remote-access.controller';
import { RemoteAccessGateway } from './remote-access.gateway';
import { RemoteAccessService } from './remote-access.service';

@Module({ imports: [OperationLogsModule], controllers: [RemoteAccessController], providers: [RemoteAccessGateway, RemoteAccessService], exports: [RemoteAccessGateway, RemoteAccessService] })
export class RemoteAccessModule {}

import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { ProductsController } from './products.controller';
import { ProductsService } from './products.service';

@Module({ imports: [OperationLogsModule], controllers: [ProductsController], providers: [ProductsService] })
export class ProductsModule {}

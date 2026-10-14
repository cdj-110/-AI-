import { Module } from '@nestjs/common';
import { FactoryGatewaysModule } from '../factory-gateways/factory-gateways.module';
import { LabelPrintController } from './label-print.controller';
import { LabelPrintService } from './label-print.service';

@Module({
  imports: [FactoryGatewaysModule],
  controllers: [LabelPrintController],
  providers: [LabelPrintService],
})
export class LabelPrintModule {}

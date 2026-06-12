import { Module } from '@nestjs/common';
import { ModelTemplatesController } from './model-templates.controller';
import { ModelTemplatesService } from './model-templates.service';

@Module({
  controllers: [ModelTemplatesController],
  providers: [ModelTemplatesService],
})
export class ModelTemplatesModule {}

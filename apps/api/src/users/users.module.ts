import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { UsersController } from './users.controller';
import { UsersService } from './users.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [OperationLogsModule, SubscriptionsModule],
  controllers: [UsersController],
  providers: [UsersService],
})
export class UsersModule {}

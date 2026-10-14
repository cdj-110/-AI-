import { Module } from '@nestjs/common';
import { OperationLogsModule } from '../operation-logs/operation-logs.module';
import { SubscriptionLimitsService } from './subscription-limits.service';
import { SubscriptionsController } from './subscriptions.controller';
import { SubscriptionsService } from './subscriptions.service';
import { SubscriptionLifecycleService } from './subscription-lifecycle.service';
import { AlipayPaymentService } from './alipay-payment.service';
import { AlipayNotifyController } from './alipay-notify.controller';
import { HistoryCleanupService } from './history-cleanup.service';

@Module({
  imports: [OperationLogsModule],
  controllers: [SubscriptionsController, AlipayNotifyController],
  providers: [SubscriptionsService, SubscriptionLimitsService, SubscriptionLifecycleService, AlipayPaymentService, HistoryCleanupService],
  exports: [SubscriptionLimitsService],
})
export class SubscriptionsModule {}

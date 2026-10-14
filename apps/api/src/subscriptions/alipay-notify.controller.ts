import { Body, Controller, Post, Res } from '@nestjs/common';
import type { Response } from 'express';
import { RawResponse } from '../common/decorators/raw-response.decorator';
import { SubscriptionsService } from './subscriptions.service';

@Controller('payments/alipay')
@RawResponse()
export class AlipayNotifyController {
  constructor(private readonly subscriptions: SubscriptionsService) {}

  @Post('notify')
  async notify(@Body() payload: Record<string, unknown>, @Res() response: Response) {
    try {
      await this.subscriptions.processAlipayNotification(payload);
      response.status(200).type('text/plain').send('success');
    } catch (error) {
      console.error('Alipay notification rejected:', error instanceof Error ? error.message : error);
      response.status(200).type('text/plain').send('failure');
    }
  }
}

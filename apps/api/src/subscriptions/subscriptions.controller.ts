import { Body, Controller, Get, Headers, Param, Patch, Post, Query, Res, StreamableFile, UploadedFile, UseGuards, UseInterceptors } from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { createReadStream } from 'node:fs';
import type { Response } from 'express';
import { AuthUser, CurrentUser } from '../common/decorators/current-user.decorator';
import { RawResponse } from '../common/decorators/raw-response.decorator';
import { Roles } from '../common/decorators/roles.decorator';
import { JwtAuthGuard } from '../common/guards/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { CreateSubscriptionPlanDto, ReviewSubscriptionOrderDto, SaveQuotaAddonDto, SubmitPlanUpgradeDto, SubmitSubscriptionOrderDto, UpdateSubscriptionPlanDto, UpgradeRequestQueryDto } from './dto/subscription.dto';
import { SubscriptionsService } from './subscriptions.service';
import { HistoryCleanupService } from './history-cleanup.service';

@Controller('subscriptions')
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('SUPER_ADMIN', 'TENANT_ADMIN')
export class SubscriptionsController {
  constructor(private readonly subscriptions: SubscriptionsService, private readonly historyCleanup: HistoryCleanupService) {}

  @Get('customers/:tenantId/history-cleanup-impact')
  @Roles('SUPER_ADMIN')
  async cleanupImpact(@Param('tenantId') tenantId: string) {
    const { ids, ...report } = await this.historyCleanup.impact(tenantId);
    return report;
  }

  @Get('plans')
  plans(@CurrentUser() actor: AuthUser) { return this.subscriptions.listPlans(actor); }

  @Post('plans')
  @Roles('SUPER_ADMIN')
  createPlan(@CurrentUser() actor: AuthUser, @Body() dto: CreateSubscriptionPlanDto) { return this.subscriptions.createPlan(actor, dto); }

  @Patch('plans/:id')
  @Roles('SUPER_ADMIN')
  updatePlan(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: UpdateSubscriptionPlanDto) { return this.subscriptions.updatePlan(actor, id, dto); }

  @Get('addons')
  addons(@CurrentUser() actor: AuthUser) { return this.subscriptions.listAddons(actor); }

  @Get('admin-notifications')
  @Roles('SUPER_ADMIN')
  adminNotifications(@CurrentUser() actor: AuthUser) { return this.subscriptions.adminNotifications(actor); }

  @Patch('admin-notifications/read')
  @Roles('SUPER_ADMIN')
  markAdminNotificationsRead(@CurrentUser() actor: AuthUser) { return this.subscriptions.markAdminNotificationsRead(actor); }

  @Get('customers')
  @Roles('SUPER_ADMIN')
  customers() { return this.subscriptions.customerSubscriptions(); }

  @Post('customers/:tenantId/orders')
  @Roles('SUPER_ADMIN')
  @UseInterceptors(FileInterceptor('voucher', { limits: { fileSize: 8 * 1024 * 1024, files: 1 } }))
  createCustomerOrder(
    @CurrentUser() actor: AuthUser,
    @Param('tenantId') tenantId: string,
    @Body() dto: SubmitSubscriptionOrderDto,
    @UploadedFile() file?: { buffer: Buffer; mimetype: string; originalname: string; size: number },
  ) {
    return this.subscriptions.submitOrderForTenant(actor, tenantId, dto, file);
  }

  @Post('addons')
  @Roles('SUPER_ADMIN')
  createAddon(@CurrentUser() actor: AuthUser, @Body() dto: SaveQuotaAddonDto) { return this.subscriptions.createAddon(actor, dto); }

  @Patch('addons/:id')
  @Roles('SUPER_ADMIN')
  updateAddon(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: SaveQuotaAddonDto) { return this.subscriptions.updateAddon(actor, id, dto); }

  @Get('current')
  @Roles('TENANT_ADMIN')
  current(@CurrentUser() actor: AuthUser) { return this.subscriptions.current(actor); }

  @Get('upgrade-requests/mine')
  @Roles('TENANT_ADMIN')
  mine(@CurrentUser() actor: AuthUser) { return this.subscriptions.myRequests(actor); }

  @Post('upgrade-requests')
  @Roles('TENANT_ADMIN')
  @UseInterceptors(FileInterceptor('voucher', { limits: { fileSize: 8 * 1024 * 1024, files: 1 } }))
  submit(@CurrentUser() actor: AuthUser, @Body() dto: SubmitPlanUpgradeDto, @UploadedFile() file?: { buffer: Buffer; mimetype: string; originalname: string; size: number }) {
    return this.subscriptions.submit(actor, dto, file);
  }

  @Post('orders')
  @Roles('TENANT_ADMIN')
  @UseInterceptors(FileInterceptor('voucher', { limits: { fileSize: 8 * 1024 * 1024, files: 1 } }))
  createOrder(@CurrentUser() actor: AuthUser, @Body() dto: SubmitSubscriptionOrderDto, @UploadedFile() file?: { buffer: Buffer; mimetype: string; originalname: string; size: number }) {
    return this.subscriptions.submitOrder(actor, dto, file);
  }

  @Post('orders/alipay')
  @Roles('TENANT_ADMIN')
  createAlipayOrder(@CurrentUser() actor: AuthUser, @Body() dto: SubmitSubscriptionOrderDto, @Headers('origin') origin?: string) {
    return this.subscriptions.createAlipayOrder(actor, dto, origin);
  }

  @Post('orders/:id/alipay/checkout')
  @Roles('TENANT_ADMIN')
  alipayCheckout(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Headers('origin') origin?: string) {
    return this.subscriptions.alipayCheckout(actor, id, origin);
  }

  @Get('orders/:id/payment-status')
  @Roles('TENANT_ADMIN')
  alipayPaymentStatus(@CurrentUser() actor: AuthUser, @Param('id') id: string) {
    return this.subscriptions.alipayPaymentStatus(actor, id);
  }

  @Get('upgrade-requests')
  @Roles('SUPER_ADMIN')
  requests(@Query() query: UpgradeRequestQueryDto) { return this.subscriptions.listRequests(query); }

  @Post('upgrade-requests/:id/review')
  @Roles('SUPER_ADMIN')
  review(@CurrentUser() actor: AuthUser, @Param('id') id: string, @Body() dto: ReviewSubscriptionOrderDto) { return this.subscriptions.review(actor, id, dto); }

  @Get('upgrade-requests/:id/voucher')
  @Roles('SUPER_ADMIN')
  @RawResponse()
  async voucher(@Param('id') id: string, @Res({ passthrough: true }) response: Response) {
    const file = await this.subscriptions.voucher(id);
    response.setHeader('Content-Type', file.voucherMimeType);
    response.setHeader('Content-Disposition', `inline; filename*=UTF-8''${encodeURIComponent(file.voucherOriginalName)}`);
    response.setHeader('Cache-Control', 'private, no-store');
    return new StreamableFile(createReadStream(file.absolutePath));
  }

  @Patch('notifications/read')
  @Roles('TENANT_ADMIN')
  markNotificationsRead(@CurrentUser() actor: AuthUser) { return this.subscriptions.markNotificationsRead(actor); }

  @Get('notifications')
  @Roles('TENANT_ADMIN')
  notifications(@CurrentUser() actor: AuthUser) { return this.subscriptions.notifications(actor); }
}

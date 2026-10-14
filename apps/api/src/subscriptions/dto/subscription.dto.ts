import { Transform, Type } from 'class-transformer';
import { ArrayNotEmpty, IsArray, IsBoolean, IsDateString, IsIn, IsInt, IsOptional, IsString, Length, Max, Min } from 'class-validator';

export class CreateSubscriptionPlanDto {
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(3660) historyRetentionDays?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(120) historyRetentionMonths?: number;
  @IsString() @Length(2, 40) code!: string;
  @IsString() @Length(2, 60) name!: string;
  @IsOptional() @IsString() @Length(0, 300) description?: string;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) deviceLimit!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(100_000_000) pointLimit!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) scadaProjectLimit!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) userLimit!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000_000) monthlyPriceCents!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000_000) priceCents!: number;
  @IsArray() @ArrayNotEmpty() @IsIn(['MONTHLY', 'YEARLY'], { each: true }) billingCycles!: Array<'MONTHLY' | 'YEARLY'>;
  @IsOptional() @IsBoolean() quoteOnly?: boolean;
  @IsOptional() @IsBoolean() enabled?: boolean;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(10000) sortOrder?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(90) gracePeriodDays?: number;
}

export class UpdateSubscriptionPlanDto {
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(3660) historyRetentionDays?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(120) historyRetentionMonths?: number;
  @IsOptional() @IsString() @Length(2, 40) code?: string;
  @IsOptional() @IsString() @Length(2, 60) name?: string;
  @IsOptional() @IsString() @Length(0, 300) description?: string;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) deviceLimit?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(100_000_000) pointLimit?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) scadaProjectLimit?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000) userLimit?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000_000) monthlyPriceCents?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000_000) priceCents?: number;
  @IsOptional() @IsArray() @ArrayNotEmpty() @IsIn(['MONTHLY', 'YEARLY'], { each: true }) billingCycles?: Array<'MONTHLY' | 'YEARLY'>;
  @IsOptional() @IsBoolean() quoteOnly?: boolean;
  @IsOptional() @IsBoolean() enabled?: boolean;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(10000) sortOrder?: number;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(90) gracePeriodDays?: number;
}

export class SaveQuotaAddonDto {
  @IsString() @Length(2, 40) code!: string;
  @IsString() @Length(2, 80) name!: string;
  @IsOptional() @IsString() @Length(0, 300) description?: string;
  @IsIn(['DEVICE', 'POINT', 'SCADA', 'USER', 'HISTORY']) resourceType!: string;
  @IsOptional() @IsBoolean() pricePending?: boolean;
  @Type(() => Number) @IsInt() @Min(1) @Max(100_000_000) quantity!: number;
  @Type(() => Number) @IsInt() @Min(0) @Max(1_000_000_000) priceCents!: number;
  @IsOptional() @IsBoolean() quoteOnly?: boolean;
  @IsOptional() @IsBoolean() enabled?: boolean;
  @IsOptional() @Type(() => Number) @IsInt() @Min(0) @Max(10000) sortOrder?: number;
}

export class SubmitSubscriptionOrderDto {
  @IsIn(['UPGRADE', 'RENEW', 'EXPANSION']) orderType!: 'UPGRADE' | 'RENEW' | 'EXPANSION';
  @IsOptional() @IsString() targetPlanId?: string;
  @IsOptional() @IsString() addonId?: string;
  @IsOptional() @IsIn(['MONTHLY', 'YEARLY']) billingCycle?: 'MONTHLY' | 'YEARLY';
  @IsOptional() @Type(() => Number) @IsInt() @Min(1) @Max(100) addonQuantity?: number;
  @IsOptional() @IsString() @Length(0, 100) paymentReference?: string;
  @IsOptional() @IsString() @Length(0, 100) paidBy?: string;
  @IsOptional() @IsDateString() paidAt?: string;
  @IsOptional() @IsIn(['MANUAL', 'ALIPAY']) paymentMethod?: 'MANUAL' | 'ALIPAY';
}

export class SubmitPlanUpgradeDto {
  @IsString() targetPlanId!: string;
  @IsOptional() @IsIn(['MONTHLY', 'YEARLY']) billingCycle?: 'MONTHLY' | 'YEARLY';
}

export class ReviewSubscriptionOrderDto {
  @IsIn(['APPROVED', 'REJECTED']) decision!: 'APPROVED' | 'REJECTED';
  @IsOptional() @IsString() @Length(0, 500) note?: string;
}

export class UpgradeRequestQueryDto {
  @Transform(({ value }) => value === '' ? undefined : value)
  @IsOptional() @IsIn(['PENDING_PAYMENT', 'PENDING_REVIEW', 'APPROVED', 'REJECTED', 'PAYMENT_CLOSED', 'PAYMENT_FAILED']) status?: string;
  @IsOptional() @IsString() keyword?: string;
  @Transform(({ value }) => value === '' ? undefined : value)
  @IsOptional() @IsIn(['UPGRADE', 'RENEW', 'EXPANSION']) orderType?: string;
  @IsOptional() @Transform(({ value }) => Math.max(1, Number(value) || 1)) page = 1;
  @IsOptional() @Transform(({ value }) => Math.min(100, Math.max(1, Number(value) || 20))) pageSize = 20;
}

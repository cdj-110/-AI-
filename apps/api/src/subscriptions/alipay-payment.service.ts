import { BadGatewayException, BadRequestException, Injectable, ServiceUnavailableException } from '@nestjs/common';
import { createSign, createVerify, createPrivateKey, createPublicKey } from 'node:crypto';
import { readFileSync } from 'node:fs';

export interface AlipayCheckout {
  gateway: string;
  method: 'POST';
  fields: Record<string, string>;
  sandbox: boolean;
  paymentAmountCents: number;
}

export interface AlipayTradeResult {
  code?: string;
  msg?: string;
  sub_code?: string;
  sub_msg?: string;
  out_trade_no?: string;
  trade_no?: string;
  trade_status?: string;
  total_amount?: string;
  buyer_logon_id?: string;
}

@Injectable()
export class AlipayPaymentService {
  private readonly enabled = String(process.env.ALIPAY_ENABLED ?? 'false').toLowerCase() === 'true';
  private readonly sandbox = String(process.env.ALIPAY_SANDBOX ?? 'true').toLowerCase() === 'true';
  private readonly appId = process.env.ALIPAY_APP_ID ?? '';
  private readonly sellerId = process.env.ALIPAY_SELLER_ID ?? '';
  private readonly gateway = process.env.ALIPAY_GATEWAY_URL ?? 'https://openapi-sandbox.dl.alipaydev.com/gateway.do';
  private readonly notifyUrl = process.env.ALIPAY_NOTIFY_URL ?? '';

  available() {
    try { this.keys(); return this.enabled && Boolean(this.appId && this.gateway); } catch { return false; }
  }

  isSandbox() { return this.sandbox; }

  paymentAmount(orderAmountCents: number) {
    if (this.sandbox) {
      if (!this.gateway.includes('alipaydev.com')) throw new BadRequestException('沙箱金额覆盖只能用于支付宝沙箱网关');
      return Math.max(1, Number(process.env.ALIPAY_SANDBOX_TEST_AMOUNT_CENTS ?? 1));
    }
    return orderAmountCents;
  }

  checkout(order: { orderNo: string; paymentAmountCents: number | null; amountCents: number }, subject: string, returnUrl?: string): AlipayCheckout {
    this.assertConfigured();
    const amount = order.paymentAmountCents ?? order.amountCents;
    if (amount < 1) throw new BadRequestException('支付宝支付金额不能低于 0.01 元');
    const bizContent = {
      out_trade_no: order.orderNo,
      total_amount: (amount / 100).toFixed(2),
      subject: this.safeSubject(subject),
      product_code: 'FAST_INSTANT_TRADE_PAY',
    };
    const fields = this.signedParameters('alipay.trade.page.pay', bizContent, {
      ...(this.notifyUrl ? { notify_url: this.notifyUrl } : {}),
      ...(returnUrl ? { return_url: returnUrl } : {}),
    });
    // 电脑网站支付的浏览器表单要求 charset 位于网关 URL，而不是 POST 表单体。
    // charset 仍参与签名，支付宝会合并查询参数与表单字段后验签。
    delete fields.charset;
    const gateway = new URL(this.gateway);
    gateway.searchParams.set('charset', 'utf-8');
    return { gateway: gateway.toString(), method: 'POST', fields, sandbox: this.sandbox, paymentAmountCents: amount };
  }

  async query(orderNo: string): Promise<AlipayTradeResult> {
    this.assertConfigured();
    const fields = this.signedParameters('alipay.trade.query', { out_trade_no: orderNo });
    let response: Response;
    try {
      response = await fetch(this.gateway, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8' },
        body: new URLSearchParams(fields),
        signal: AbortSignal.timeout(10_000),
      });
    } catch (error) {
      throw new BadGatewayException(error instanceof Error ? `支付宝沙箱连接失败：${error.message}` : '支付宝沙箱连接失败');
    }
    const raw = await response.text();
    if (!response.ok) throw new BadGatewayException(`支付宝沙箱响应异常：HTTP ${response.status}`);
    let envelope: Record<string, unknown>;
    try { envelope = JSON.parse(raw) as Record<string, unknown>; } catch { throw new BadGatewayException('支付宝沙箱返回了无效数据'); }
    const result = envelope.alipay_trade_query_response as AlipayTradeResult | undefined;
    const signature = typeof envelope.sign === 'string' ? envelope.sign : '';
    if (!result || !signature) throw new BadGatewayException('支付宝沙箱响应缺少验签数据');
    const signedContent = this.extractResponseNode(raw, 'alipay_trade_query_response');
    if (!this.verifyText(signedContent, signature)) throw new BadGatewayException('支付宝查询响应验签失败');
    return result;
  }

  verifyNotification(payload: Record<string, unknown>) {
    this.assertConfigured();
    const normalized = Object.fromEntries(Object.entries(payload).map(([key, value]) => [key, Array.isArray(value) ? String(value[0] ?? '') : String(value ?? '')]));
    const signature = normalized.sign;
    if (!signature) return false;
    const content = Object.entries(normalized)
      .filter(([key, value]) => key !== 'sign' && key !== 'sign_type' && value !== '')
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]) => `${key}=${value}`)
      .join('&');
    return this.verifyText(content, signature);
  }

  validateNotificationIdentity(payload: Record<string, unknown>) {
    if (String(payload.app_id ?? '') !== this.appId) throw new BadRequestException('支付宝回调应用标识不匹配');
    if (this.sellerId && String(payload.seller_id ?? '') !== this.sellerId) throw new BadRequestException('支付宝回调商户标识不匹配');
  }

  private signedParameters(method: string, bizContent: Record<string, unknown>, extra: Record<string, string> = {}) {
    const params: Record<string, string> = {
      app_id: this.appId,
      method,
      format: 'JSON',
      charset: 'utf-8',
      sign_type: 'RSA2',
      timestamp: this.timestamp(),
      version: '1.0',
      biz_content: JSON.stringify(bizContent),
      ...extra,
    };
    const content = Object.entries(params).filter(([, value]) => value !== '').sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => `${key}=${value}`).join('&');
    const signer = createSign('RSA-SHA256');
    signer.update(content, 'utf8');
    signer.end();
    params.sign = signer.sign(this.keys().privateKey, 'base64');
    return params;
  }

  private verifyText(content: string, signature: string) {
    const verifier = createVerify('RSA-SHA256');
    verifier.update(content, 'utf8');
    verifier.end();
    return verifier.verify(this.keys().alipayPublicKey, signature, 'base64');
  }

  private keys() {
    const privatePath = process.env.ALIPAY_APP_PRIVATE_KEY_FILE ?? '';
    const publicPath = process.env.ALIPAY_PUBLIC_KEY_FILE ?? '';
    if (!privatePath || !publicPath) throw new ServiceUnavailableException('支付宝沙箱密钥路径未配置');
    return {
      privateKey: createPrivateKey(readFileSync(privatePath)),
      alipayPublicKey: createPublicKey(readFileSync(publicPath)),
    };
  }

  private assertConfigured() {
    if (!this.enabled) throw new ServiceUnavailableException('支付宝沙箱支付尚未启用');
    if (!this.appId) throw new ServiceUnavailableException('支付宝沙箱 APPID 未配置');
    this.keys();
  }

  private timestamp() {
    const china = new Date(Date.now() + 8 * 60 * 60 * 1000);
    return china.toISOString().slice(0, 19).replace('T', ' ');
  }

  private safeSubject(value: string) {
    return value.replace(/[\/=&]/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 128) || '微控物联云平台订单';
  }

  private extractResponseNode(raw: string, key: string) {
    const marker = `"${key}":`;
    const markerIndex = raw.indexOf(marker);
    if (markerIndex < 0) throw new BadGatewayException('支付宝响应结构无效');
    const start = raw.indexOf('{', markerIndex + marker.length);
    if (start < 0) throw new BadGatewayException('支付宝响应结构无效');
    let depth = 0;
    let quoted = false;
    let escaped = false;
    for (let index = start; index < raw.length; index += 1) {
      const char = raw[index];
      if (quoted) {
        if (escaped) escaped = false;
        else if (char === '\\') escaped = true;
        else if (char === '"') quoted = false;
        continue;
      }
      if (char === '"') quoted = true;
      else if (char === '{') depth += 1;
      else if (char === '}' && --depth === 0) return raw.slice(start, index + 1);
    }
    throw new BadGatewayException('支付宝响应结构不完整');
  }
}

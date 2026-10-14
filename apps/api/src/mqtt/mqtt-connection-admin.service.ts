import { Injectable, Logger } from '@nestjs/common';
import { readFileSync } from 'node:fs';

export type MqttDisconnectResult = 'disconnected' | 'not_found' | 'unavailable';

@Injectable()
export class MqttConnectionAdminService {
  private readonly logger = new Logger(MqttConnectionAdminService.name);

  async disconnect(clientId: string): Promise<MqttDisconnectResult> {
    const baseUrl = process.env.EMQX_API_URL?.replace(/\/$/, '');
    try {
      const credentialsFile = process.env.EMQX_API_CREDENTIALS_FILE?.trim();
      let apiKey = process.env.EMQX_API_KEY;
      let apiSecret = process.env.EMQX_API_SECRET;
      if (credentialsFile) {
        // A configured file takes precedence and must fail closed if missing/invalid.
        try {
          const credentials = JSON.parse(readFileSync(credentialsFile, 'utf8'));
          apiKey = typeof credentials?.api_key === 'string' ? credentials.api_key : undefined;
          apiSecret = typeof credentials?.api_secret === 'string' ? credentials.api_secret : undefined;
        } catch {
          this.logger.warn('EMQX 专用 API 密钥文件不可用，无法主动断开连接');
          return 'unavailable';
        }
      }
      if (!baseUrl || !apiKey?.trim() || !apiSecret?.trim()) {
        this.logger.warn('EMQX 专用 API 密钥未配置，无法主动断开连接');
        return 'unavailable';
      }
      // Never fall back to Dashboard login: batch disconnects must not lock out administrators.
      const authorization = `Basic ${Buffer.from(`${apiKey}:${apiSecret}`).toString('base64')}`;
      const response = await fetch(`${baseUrl}/clients/${encodeURIComponent(clientId)}`, {
        signal: AbortSignal.timeout(5000),
        method: 'DELETE',
        headers: { Authorization: authorization },
      });
      if (response.ok) return 'disconnected';
      if (response.status === 404) return 'not_found';
      if (response.status === 401 || response.status === 403) {
        this.logger.warn(`EMQX 专用 API 密钥无效、已过期或权限不足（HTTP ${response.status}）`);
        return 'unavailable';
      }
      this.logger.warn(`断开 MQTT 客户端 ${clientId} 失败，EMQX 返回 ${response.status}`);
      return 'unavailable';
    } catch (error) {
      this.logger.warn(`断开 MQTT 客户端 ${clientId} 失败：${error instanceof Error ? error.message : String(error)}`);
      return 'unavailable';
    }
  }
}

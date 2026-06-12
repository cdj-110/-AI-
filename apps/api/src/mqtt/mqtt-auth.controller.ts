import { Body, Controller, HttpCode, Post } from '@nestjs/common';
import * as bcrypt from 'bcryptjs';
import { RawResponse } from '../common/decorators/raw-response.decorator';
import { PrismaService } from '../prisma/prisma.service';

type MqttDecision = { result: 'allow' | 'deny'; is_superuser?: boolean };

interface MqttAuthBody {
  clientid?: string;
  username?: string;
  password?: string;
}

interface MqttAclBody {
  clientid?: string;
  username?: string;
  action?: string;
  topic?: string;
}

@Controller('mqtt')
@RawResponse()
export class MqttAuthController {
  constructor(private readonly prisma: PrismaService) {}

  // EMQX HTTP Auth 回调：平台采集端和真实设备都走这里校验用户名/密码。
  @Post('auth')
  @HttpCode(200)
  async auth(@Body() body: MqttAuthBody): Promise<MqttDecision> {
    const username = body.username ?? '';
    const password = body.password ?? '';
    if (process.env.MQTT_AUTH_DEBUG === 'true') {
      console.log('[mqtt-auth] auth request', { clientid: body.clientid, username, hasPassword: Boolean(password) });
    }

    // ingest 是平台内部订阅者，只允许使用专用账号，不绑定具体设备。
    if (username === (process.env.MQTT_INGEST_USERNAME ?? 'platform-ingest')) {
      return password === (process.env.MQTT_INGEST_PASSWORD ?? 'platform-ingest-secret')
        ? { result: 'allow', is_superuser: false }
        : { result: 'deny' };
    }

    // 真实设备必须同时匹配 username、clientId 和 bcrypt 密码，避免串用其他设备凭证。
    const device = await this.prisma.device.findUnique({ where: { mqttUsername: username } });
    if (!device || body.clientid !== device.mqttClientId) return { result: 'deny' };

    try {
      return (await bcrypt.compare(password, device.mqttPasswordHash))
        ? { result: 'allow', is_superuser: false }
        : { result: 'deny' };
    } catch {
      return { result: 'deny' };
    }
  }

  // EMQX HTTP ACL 回调：认证通过后，还要限制客户端能发布/订阅哪些 Topic。
  @Post('acl')
  @HttpCode(200)
  async acl(@Body() body: MqttAclBody): Promise<MqttDecision> {
    const username = body.username ?? '';
    const action = (body.action ?? '').toLowerCase();
    const topic = body.topic ?? '';
    if (process.env.MQTT_AUTH_DEBUG === 'true') {
      console.log('[mqtt-auth] acl request', { clientid: body.clientid, username, action, topic });
    }

    if (username === (process.env.MQTT_INGEST_USERNAME ?? 'platform-ingest')) {
      return this.canIngest(action, topic) ? { result: 'allow' } : { result: 'deny' };
    }

    const device = await this.prisma.device.findUnique({
      where: { mqttUsername: username },
      select: { id: true, deviceKey: true, deviceType: true, mqttClientId: true },
    });
    if (!device || body.clientid !== device.mqttClientId) return { result: 'deny' };

    // 直连设备只能发布自己的心跳和遥测 Topic。
    const allowedPublishTopics = new Set([
      `weikong/devices/${device.deviceKey}/heartbeat`,
      `weikong/devices/${device.deviceKey}/telemetry`,
    ]);
    if (action !== 'publish') return { result: 'deny' };
    if (allowedPublishTopics.has(topic)) return { result: 'allow' };
    if (await this.canGatewayPublishChild(device, topic)) return { result: 'allow' };
    return { result: 'deny' };
  }

  private canIngest(action: string, topic: string) {
    // ingest 只允许订阅平台需要消费的系统事件和设备上报主题。
    if (action !== 'subscribe') return false;
    return topic === '$SYS/#'
      || topic === '$SYS/brokers/+/clients/+/connected'
      || topic === '$SYS/brokers/+/clients/+/disconnected'
      || topic === 'weikong/devices/+/heartbeat'
      || topic === 'weikong/devices/+/telemetry'
      || topic === 'weikong/gateways/+/children/+/heartbeat'
      || topic === 'weikong/gateways/+/children/+/telemetry';
  }

  private async canGatewayPublishChild(
    gateway: { id: string; deviceKey: string; deviceType: string },
    topic: string,
  ) {
    // 网关代发模式：只有网关设备可以向自己绑定的子设备 Topic 发布。
    if (gateway.deviceType !== 'GATEWAY') return false;
    const match = topic.match(/^weikong\/gateways\/([^/]+)\/children\/([^/]+)\/(heartbeat|telemetry)$/);
    if (!match) return false;
    const [, gatewayKey, childKey] = match;
    if (gatewayKey !== gateway.deviceKey) return false;
    const child = await this.prisma.device.findFirst({
      where: {
        deviceKey: childKey,
        gatewayId: gateway.id,
        deviceType: 'GATEWAY_CHILD',
      },
      select: { id: true },
    });
    return Boolean(child);
  }
}

const assert = require('node:assert/strict');
const { MqttAuthController } = require('../dist/mqtt/mqtt-auth.controller');

async function main() {
  let mode = 'direct-disabled';
  const prisma = {
    device: {
      findUnique: async () => {
        if (mode === 'direct-disabled') {
          return {
            id: 'device-1', deviceKey: 'disabled-device', deviceType: 'DIRECT',
            mqttClientId: 'client-1', mqttPasswordHash: 'unused', cloudAccessEnabled: false,
          };
        }
        return {
          id: 'gateway-1', deviceKey: 'gateway-1', deviceType: 'GATEWAY',
          mqttClientId: 'gateway-client', mqttPasswordHash: 'unused', cloudAccessEnabled: true,
        };
      },
      findFirst: async () => null,
    },
    factoryGateway: {
      findUnique: async () => ({
        status: 'BOUND', deviceSecretHash: 'unused', device: { cloudAccessEnabled: false },
      }),
    },
  };
  const controller = new MqttAuthController(prisma);

  assert.deepEqual(
    await controller.auth({ username: 'disabled-device', clientid: 'client-1', password: 'anything' }),
    { result: 'deny' },
    'disabled direct device must fail authentication before password comparison',
  );
  assert.deepEqual(
    await controller.acl({ username: 'disabled-device', clientid: 'client-1', action: 'publish', topic: 'weikong/devices/disabled-device/telemetry' }),
    { result: 'deny' },
    'disabled direct device must fail authorization',
  );
  assert.deepEqual(
    await controller.auth({ username: 'factory:ABC', clientid: 'factory_ABC', password: 'anything' }),
    { result: 'deny' },
    'factory identity bound to a disabled device must fail authentication',
  );

  mode = 'gateway-enabled';
  assert.deepEqual(
    await controller.acl({
      username: 'gateway-1', clientid: 'gateway-client', action: 'publish',
      topic: 'weikong/gateways/gateway-1/children/disabled-child/telemetry',
    }),
    { result: 'deny' },
    'gateway must not publish for a disabled child',
  );
  console.log('device cloud access verification passed');
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

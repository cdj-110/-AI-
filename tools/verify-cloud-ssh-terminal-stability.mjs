import { randomBytes } from 'node:crypto';
import Redis from '../apps/api/node_modules/ioredis/built/index.js';
import WebSocket from '../apps/api/node_modules/ws/wrapper.mjs';

for (const name of ['GATEWAY_DEVICE_ID', 'SSH_PASSWORD']) {
  if (!process.env[name]) throw new Error(`missing ${name}`);
}

const holdSeconds = Math.max(80, Number(process.env.TEST_SECONDS || 95));
const sessionId = randomBytes(24).toString('base64url');
const token = randomBytes(32).toString('base64url');
const prefix = process.env.REMOTE_ACCESS_SESSION_PREFIX || 'weikong:remote-access';
const now = new Date();
const session = {
  id: sessionId,
  token,
  deviceId: process.env.GATEWAY_DEVICE_ID,
  userId: 'terminal-stability-check',
  tenantId: '',
  mode: 'ssh',
  createdAt: now.toISOString(),
  expiresAt: new Date(now.getTime() + 300_000).toISOString(),
};
const redis = new Redis({ host: '127.0.0.1', port: 6379, maxRetriesPerRequest: 2 });
await redis.multi()
  .set(`${prefix}:token:${token}`, JSON.stringify(session), 'EX', 300)
  .set(`${prefix}:session:${sessionId}`, JSON.stringify(session), 'EX', 300)
  .exec();

const startedAt = Date.now();
let ticks = 0;
let output = '';
let ticker;
let finishTimer;
try {
  const result = await new Promise((resolve, reject) => {
    const socket = new WebSocket(`ws://127.0.0.1:3201/terminal/connect/${token}`);
    const timeout = setTimeout(() => {
      socket.close();
      reject(new Error('terminal stability verification timed out'));
    }, (holdSeconds + 30) * 1000);
    const fail = (error) => {
      clearTimeout(timeout);
      clearInterval(ticker);
      clearTimeout(finishTimer);
      reject(error);
    };
    socket.on('message', (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type === 'terminal.ready') {
        socket.send(JSON.stringify({
          type: 'ssh.auth',
          username: process.env.SSH_USER || 'root',
          password: process.env.SSH_PASSWORD,
          cols: 100,
          rows: 30,
        }));
      } else if (message.type === 'ssh.opened') {
        const sendCommand = (command) => socket.send(JSON.stringify({
          type: 'ssh.input', dataBase64: Buffer.from(`${command}\r`).toString('base64'),
        }));
        sendCommand("printf '__WK_STABILITY_START__\\n'");
        ticker = setInterval(() => {
          ticks += 1;
          sendCommand(`printf '__WK_TICK_${ticks}__\\n'`);
        }, 10_000);
        finishTimer = setTimeout(() => {
          sendCommand("printf '__WK_STABILITY_END__\\n'; uname -m; exit");
        }, holdSeconds * 1000);
      } else if (message.type === 'ssh.data') {
        output += Buffer.from(message.dataBase64, 'base64').toString('utf8');
      } else if (message.type === 'ssh.error') {
        socket.close();
        fail(new Error(message.error || 'ssh error'));
      } else if (message.type === 'ssh.close') {
        clearTimeout(timeout);
        clearInterval(ticker);
        clearTimeout(finishTimer);
        if (!output.includes('__WK_STABILITY_START__') || !output.includes('__WK_STABILITY_END__')) {
          reject(new Error(`terminal closed before stability markers completed: ${output.slice(-500)}`));
        } else {
          resolve({ stable: true, durationSeconds: Math.round((Date.now() - startedAt) / 1000), ticks });
        }
        socket.close();
      }
    });
    socket.on('close', (code, reason) => {
      if (!output.includes('__WK_STABILITY_END__')) {
        fail(new Error(`terminal disconnected early: code=${code} reason=${reason.toString()}`));
      }
    });
    socket.on('error', fail);
  });
  console.log(JSON.stringify(result));
} finally {
  clearInterval(ticker);
  clearTimeout(finishTimer);
  await redis.del(`${prefix}:token:${token}`, `${prefix}:session:${sessionId}`);
  await redis.quit();
}

import WebSocket from '../apps/api/node_modules/ws/wrapper.mjs';

const required = ['CLOUD_USER', 'CLOUD_PASSWORD', 'GATEWAY_DEVICE_ID', 'SSH_USER', 'SSH_PASSWORD'];
for (const name of required) {
  if (!process.env[name]) throw new Error(`missing ${name}`);
}

const apiBase = process.env.CLOUD_API_BASE || 'http://127.0.0.1:3100';
const loginResponse = await fetch(`${apiBase}/api/auth/login/admin`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: process.env.CLOUD_USER, password: process.env.CLOUD_PASSWORD }),
});
if (!loginResponse.ok) throw new Error(`cloud login failed: HTTP ${loginResponse.status}`);
const loginPayload = await loginResponse.json();
const login = loginPayload.data ?? loginPayload;

const sessionResponse = await fetch(`${apiBase}/api/remote-access/devices/${process.env.GATEWAY_DEVICE_ID}/terminal-sessions`, {
  method: 'POST',
  headers: { Authorization: `Bearer ${login.accessToken}` },
});
if (!sessionResponse.ok) throw new Error(`terminal session failed: HTTP ${sessionResponse.status} ${await sessionResponse.text()}`);
const sessionPayload = await sessionResponse.json();
const session = sessionPayload.data ?? sessionPayload;

const marker = '__WK_CLOUD_SSH_OK__';
const result = await new Promise((resolve, reject) => {
  const socket = new WebSocket(session.url);
  let output = '';
  const timer = setTimeout(() => {
    socket.close();
    reject(new Error('terminal verification timed out'));
  }, 30_000);
  socket.on('message', (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === 'terminal.ready') {
      socket.send(JSON.stringify({
        type: 'ssh.auth', username: process.env.SSH_USER, password: process.env.SSH_PASSWORD, cols: 100, rows: 30,
      }));
    } else if (message.type === 'ssh.opened') {
      const command = `printf '${marker}\\n'; uname -m; exit\r`;
      socket.send(JSON.stringify({ type: 'ssh.input', dataBase64: Buffer.from(command).toString('base64') }));
    } else if (message.type === 'ssh.data') {
      output += Buffer.from(message.dataBase64, 'base64').toString('utf8');
    } else if (message.type === 'ssh.error') {
      clearTimeout(timer);
      reject(new Error(message.error || 'ssh error'));
      socket.close();
    } else if (message.type === 'ssh.close') {
      clearTimeout(timer);
      if (!output.includes(marker)) reject(new Error(`terminal marker missing: ${output.slice(-400)}`));
      else resolve({ connected: true, commandOutputReceived: true, architecture: output.match(/armv\d+l|aarch64|x86_64/)?.[0] ?? 'unknown' });
      socket.close();
    }
  });
  socket.on('error', (error) => {
    clearTimeout(timer);
    reject(error);
  });
});

console.log(JSON.stringify(result));

const path = require('node:path');
const { PrismaClient } = require(path.join(__dirname, '..', 'apps', 'api', 'node_modules', '@prisma', 'client'));

const allowedSurfaces = new Set(['ADMIN', 'TENANT', 'GATEWAY']);
const allowedCategories = new Set(['FEATURE', 'IMPROVEMENT', 'FIX', 'SECURITY']);

async function readInput() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const raw = Buffer.concat(chunks).toString('utf8').trim();
  if (!raw) throw new Error('缺少更新日志 JSON 输入');
  return JSON.parse(raw);
}

function normalize(input) {
  const date = String(input.date || '').trim();
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new Error('date 必须为 YYYY-MM-DD');
  const title = String(input.title || '').trim();
  if (!title || title.length > 120) throw new Error('title 长度必须为 1-120');
  const summary = String(input.summary || '').trim();
  const surfaces = [...new Set(Array.isArray(input.surfaces) ? input.surfaces : [])].filter(item => allowedSurfaces.has(item));
  if (!surfaces.length) throw new Error('至少需要一个有效 surfaces');
  const category = allowedCategories.has(input.category) ? input.category : 'IMPROVEMENT';
  const items = (Array.isArray(input.items) ? input.items : []).map(item => ({ text: String(item?.text || item || '').trim() })).filter(item => item.text);
  if (!items.length) throw new Error('至少需要一条有效 items');
  return {
    date,
    releaseDate: new Date(`${date}T00:00:00+08:00`),
    version: String(input.version || '').trim() || null,
    title,
    summary: summary || null,
    surfaces,
    category,
    items,
  };
}

async function main() {
  process.env.DATABASE_URL ||= 'postgresql://weikong@127.0.0.1:5432/weikong_iot?schema=public';
  const prisma = new PrismaClient();
  try {
    const input = normalize(await readInput());
    const existing = await prisma.changelogEntry.findFirst({
      where: { createdById: 'AUTOMATION', releaseDate: input.releaseDate },
      select: { id: true },
    });
    const data = {
      version: input.version,
      title: input.title,
      summary: input.summary,
      releaseDate: input.releaseDate,
      surfaces: input.surfaces,
      category: input.category,
      items: input.items,
      status: 'PUBLISHED',
      publishedAt: new Date(),
      createdByName: '每日自动更新',
    };
    const entry = existing
      ? await prisma.changelogEntry.update({ where: { id: existing.id }, data })
      : await prisma.changelogEntry.create({ data: { ...data, createdById: 'AUTOMATION' } });
    console.log(JSON.stringify({ success: true, action: existing ? 'updated' : 'created', id: entry.id, date: input.date, itemCount: input.items.length }));
  } finally {
    await prisma.$disconnect();
  }
}

main().catch(error => { console.error(error instanceof Error ? error.message : error); process.exitCode = 1; });

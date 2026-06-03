import { PrismaClient } from '@prisma/client';
import * as bcrypt from 'bcryptjs';

const prisma = new PrismaClient();

async function main() {
  const password = await bcrypt.hash('admin123456', 10);
  const defaultTenant = await prisma.tenant.findFirst({ where: { name: '系统默认租户' } });
  if (!defaultTenant) {
    await prisma.tenant.create({ data: { name: '系统默认租户' } });
  }
  await prisma.user.upsert({
    where: { username: 'admin' },
    update: { password, role: 'SUPER_ADMIN', status: 'ACTIVE' },
    create: {
      username: 'admin',
      password,
      nickname: '超级管理员',
      role: 'SUPER_ADMIN',
      status: 'ACTIVE',
    },
  });
}

main()
  .finally(async () => {
    await prisma.$disconnect();
  });

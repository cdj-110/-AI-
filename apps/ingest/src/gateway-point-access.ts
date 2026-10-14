import { Prisma } from '@prisma/client';

export function reportedPointAccess(value: { canRead?: unknown; canWrite?: unknown }) {
  return {
    ...(typeof value.canRead === 'boolean' ? { canRead: value.canRead } : {}),
    ...(typeof value.canWrite === 'boolean' ? { canWrite: value.canWrite } : {}),
  };
}

type AccessWriter = { deviceMetric: { updateMany(args: Prisma.DeviceMetricUpdateManyArgs): Promise<unknown> } };

export async function syncGatewayPointAccess(prisma: AccessWriter, deviceId: string, gatewayId: string, tenantId: string, metrics: { identifier: string; canWrite?: boolean }[]) {
  // Two bounded batches per child, not one database write per collection point.
  for (const writable of [true, false]) {
    const identifiers = metrics.filter(metric => metric.canWrite === writable).map(metric => metric.identifier);
    if (!identifiers.length) continue;
    const accessMode = writable ? 'READ_WRITE' : 'READ_ONLY';
    await prisma.deviceMetric.updateMany({
      where: { deviceId, identifier: { in: identifiers }, accessMode: { not: accessMode }, device: { gatewayId, tenantId } },
      data: { accessMode },
    });
  }
}

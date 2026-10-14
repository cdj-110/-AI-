type Raw = Record<string, unknown>;
const object = (value: unknown): Raw => value && typeof value === 'object' && !Array.isArray(value) ? value as Raw : {};
const array = (value: unknown): Raw[] => Array.isArray(value) ? value.map(object) : [];

function reportedDataType(value: unknown): { dataType?: string } {
  const type = typeof value === 'string' ? value.trim().toLowerCase() : '';
  if (['bool', 'boolean'].includes(type)) return { dataType: 'BOOLEAN' };
  if (['string', 'text'].includes(type)) return { dataType: 'STRING' };
  if (/^(number|u?int(8|16|32|64)?|float(32|64)?|double|byte|word|dword|real|lreal)$/.test(type)) return { dataType: 'NUMBER' };
  return {};
}

// null means old firmware has not reported capabilities. An empty array revokes all writes.
export function gatewayPointAccess(snapshotValue: unknown, draftItems: unknown, deviceKey: string) {
  const snapshot = object(snapshotValue), items = array(draftItems);
  const device = items.find(item => item.deviceKey === deviceKey);
  if (device?.pointAccessVersion === 1) {
    if (['CONFLICT', 'MISSING_UNREPORTED'].includes(String(device.status))) return [];
    return array(device.metrics).map(point => ({ metric: point.identifier, canRead: point.canRead === true, canWrite: point.canWrite === true, ...reportedDataType(point.dataType) }));
  }
  if (snapshot.pointAccessVersion === 1) {
    return array(snapshot.pointAccesses).filter(point => point.deviceKey === deviceKey)
      .map(point => ({ metric: point.metric, canRead: point.canRead === true, canWrite: point.canWrite === true, ...reportedDataType(point.dataType) }));
  }
  // Backwards compatible: old EG100 already reports a complete writable point list.
  if (snapshot.pointWriteVersion === 1) {
    return array(snapshot.writablePoints).filter(point => point.deviceKey === deviceKey)
      .map(point => ({ metric: point.metric, canRead: true, canWrite: true, ...reportedDataType(point.dataType) }));
  }
  return null;
}

// A tenant-wide ban never changes an individual device's own cloud permission.
export const cloudPolicyTenantSelect = { deviceCloudAccessDisabled: true } as const;

export function tenantBlocksCloud(device: { tenant?: { deviceCloudAccessDisabled: boolean } | null } | null | undefined) {
  return device?.tenant?.deviceCloudAccessDisabled === true;
}

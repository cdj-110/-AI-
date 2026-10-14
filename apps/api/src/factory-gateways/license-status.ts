export interface LicenseStatusInput {
  status?: string;
  licenseSequence?: number;
  reportedLicense?: unknown;
  licenseReportedAt?: Date | null;
  licenseActivatedAt?: Date | null;
}

export function authorizationStatus(gateway: LicenseStatusInput, now = new Date()) {
  const report = gateway.reportedLicense as Record<string, unknown> | null;
  const fresh = !!gateway.licenseReportedAt && now.getTime() - gateway.licenseReportedAt.getTime() < 90000;
  let state = 'WAITING_CONFIRMATION';
  if (gateway.status === 'DISABLED' || gateway.status === 'REMOVED') state = 'DISABLED';
  else if (report) {
    // New issuance does not imply the new file has been imported.
    if (report.licenseSequence !== gateway.licenseSequence) state = 'UPDATE_PENDING';
    else if (typeof report.expiresAt !== 'string' || (report.expiresAt !== '' && !Number.isFinite(new Date(report.expiresAt).getTime()))) state = 'INVALID';
    else if (report.expiresAt !== '' && new Date(report.expiresAt as string) <= now) state = 'EXPIRED';
    else if (report.valid !== true || report.restricted !== false) state = 'INVALID';
    else state = 'ACTIVE';
  }
  return { authorizationState: state, authorizationReportFresh: fresh };
}

import { SetMetadata } from '@nestjs/common';
import { TenantUserPermission } from '../user-permissions';

export const PERMISSIONS_KEY = 'permissions';
export const Permissions = (...permissions: TenantUserPermission[]) => SetMetadata(PERMISSIONS_KEY, permissions);

import { createParamDecorator, ExecutionContext } from '@nestjs/common';

export interface AuthUser {
  sub: string;
  sid: string;
  uv: number;
  username: string;
  role: string;
  tenantId: string | null;
  permissions?: string[];
  deviceScopeMode?: 'ALL' | 'CUSTOM';
  portal?: 'admin' | 'tenant' | 'legacy';
}

export const CurrentUser = createParamDecorator(
  (_data: unknown, context: ExecutionContext): AuthUser => {
    return context.switchToHttp().getRequest<{ user: AuthUser }>().user;
  },
);

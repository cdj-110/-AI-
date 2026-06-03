import { createParamDecorator, ExecutionContext } from '@nestjs/common';

export interface AuthUser {
  sub: string;
  username: string;
  role: string;
  tenantId: string | null;
}

export const CurrentUser = createParamDecorator(
  (_data: unknown, context: ExecutionContext): AuthUser => {
    return context.switchToHttp().getRequest<{ user: AuthUser }>().user;
  },
);

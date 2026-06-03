import { CallHandler, ExecutionContext, Injectable, NestInterceptor } from '@nestjs/common';
import { Observable, map } from 'rxjs';

@Injectable()
export class ResponseInterceptor<T> implements NestInterceptor<T, { code: number; message: string; data: T }> {
  intercept(_context: ExecutionContext, next: CallHandler<T>): Observable<{ code: number; message: string; data: T }> {
    return next.handle().pipe(map((data) => ({ code: 0, message: 'success', data })));
  }
}

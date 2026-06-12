import { CallHandler, ExecutionContext, Injectable, NestInterceptor } from '@nestjs/common';
import { Observable, map } from 'rxjs';
import { RAW_RESPONSE_KEY } from '../decorators/raw-response.decorator';

@Injectable()
export class ResponseInterceptor<T> implements NestInterceptor<T, T | { code: number; message: string; data: T }> {
  intercept(context: ExecutionContext, next: CallHandler<T>): Observable<T | { code: number; message: string; data: T }> {
    const rawResponse = Reflect.getMetadata(RAW_RESPONSE_KEY, context.getHandler())
      || Reflect.getMetadata(RAW_RESPONSE_KEY, context.getClass());
    // MQTT HTTP Auth/ACL 这类第三方回调必须返回原始结构，不能包 code/data。
    if (rawResponse) return next.handle();
    return next.handle().pipe(map((data) => ({ code: 0, message: 'success', data })));
  }
}

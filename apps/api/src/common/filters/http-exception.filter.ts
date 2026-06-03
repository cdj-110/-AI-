import { ArgumentsHost, Catch, ExceptionFilter, HttpException, HttpStatus } from '@nestjs/common';

@Catch()
export class HttpExceptionFilter implements ExceptionFilter {
  catch(exception: unknown, host: ArgumentsHost) {
    const response = host.switchToHttp().getResponse();
    const status = exception instanceof HttpException ? exception.getStatus() : HttpStatus.INTERNAL_SERVER_ERROR;
    const payload = exception instanceof HttpException ? exception.getResponse() : null;
    const message =
      typeof payload === 'object' && payload && 'message' in payload
        ? (payload as { message: string | string[] }).message
        : exception instanceof Error
          ? exception.message
          : '服务器内部错误';

    response.status(status).json({
      code: status,
      message: Array.isArray(message) ? message.join(', ') : message,
      data: null,
    });
  }
}

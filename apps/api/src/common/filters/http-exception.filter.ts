import { ArgumentsHost, Catch, ExceptionFilter, HttpException, HttpStatus } from '@nestjs/common';

@Catch()
export class HttpExceptionFilter implements ExceptionFilter {
  catch(exception: unknown, host: ArgumentsHost) {
    const response = host.switchToHttp().getResponse();
    const parserType = typeof exception === 'object' && exception && 'type' in exception ? exception.type : null;
    const status = exception instanceof HttpException ? exception.getStatus()
      : parserType === 'entity.too.large' ? HttpStatus.PAYLOAD_TOO_LARGE
      : parserType === 'entity.parse.failed' ? HttpStatus.BAD_REQUEST
      : HttpStatus.INTERNAL_SERVER_ERROR;
    const payload = exception instanceof HttpException ? exception.getResponse() : null;
    const message = parserType === 'entity.too.large' ? '请求内容过大，请减少配置或图片大小后重试'
      : parserType === 'entity.parse.failed' ? '请求内容不是有效的 JSON'
      :
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

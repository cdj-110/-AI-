import { ValidationPipe } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { HttpExceptionFilter } from './common/filters/http-exception.filter';
import { ResponseInterceptor } from './common/interceptors/response.interceptor';
import { DevicesRealtimeGateway } from './devices/devices-realtime.gateway';
import { RemoteAccessGateway } from './remote-access/remote-access.gateway';
import { RemoteAccessService } from './remote-access/remote-access.service';
import { ensureJwtSecret } from './auth/jwt-secret';
import { scadaSaveBodyParser } from './scada-projects/scada-save-body-parser';

async function bootstrap() {
  ensureJwtSecret();
  const app = await NestFactory.create(AppModule);
  app.use(scadaSaveBodyParser);
  app.setGlobalPrefix('api');
  app.enableCors();
  // whitelist 会剔除 DTO 未声明字段；新增请求字段时记得同步补 class-validator 装饰器。
  app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
  // 统一异常和成功响应结构，便于前端 apiRequest 只读取 data。
  app.useGlobalFilters(new HttpExceptionFilter());
  app.useGlobalInterceptors(new ResponseInterceptor());
  // 原生 WebSocket 与 HTTP 共用端口，客户端通过首条 auth 消息完成 JWT 鉴权。
  app.get(DevicesRealtimeGateway).attach(app.getHttpServer());
  app.get(RemoteAccessGateway).attach(app.getHttpServer());
  app.get(RemoteAccessService).startProxy();
  await app.listen(process.env.PORT ?? 3100, '0.0.0.0');
}

bootstrap();

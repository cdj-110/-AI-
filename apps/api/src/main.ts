import { ValidationPipe } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { HttpExceptionFilter } from './common/filters/http-exception.filter';
import { ResponseInterceptor } from './common/interceptors/response.interceptor';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  app.setGlobalPrefix('api');
  app.enableCors();
  // whitelist 会剔除 DTO 未声明字段；新增请求字段时记得同步补 class-validator 装饰器。
  app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
  // 统一异常和成功响应结构，便于前端 apiRequest 只读取 data。
  app.useGlobalFilters(new HttpExceptionFilter());
  app.useGlobalInterceptors(new ResponseInterceptor());
  await app.listen(process.env.PORT ?? 3100, '0.0.0.0');
}

bootstrap();

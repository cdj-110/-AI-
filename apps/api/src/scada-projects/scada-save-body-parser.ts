import { json, RequestHandler } from 'express';

// 配置最多 2 MB，缩略图最多 500 KB，另留 JSON 封装与 UTF-8 编码余量。
// 仅扩大组态保存入口；其他接口仍使用 Nest 默认请求体限制。
const parseScadaJson = json({ limit: '10mb' });
export const scadaSaveBodyParser: RequestHandler = (request, response, next) => {
  if (request.method === 'POST' && request.path === '/api/conf/config') {
    return parseScadaJson(request, response, next);
  }
  next();
};

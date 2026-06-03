# 微控物联云平台 V1.0

微控物联云平台是一个面向多租户场景的物联网管理平台。本版本完成项目骨架、账号体系、JWT 权限、多租户基础隔离、仪表盘和本地基础设施编排。

## 技术栈

- 前端：Vue 3、Vite、TypeScript、Element Plus、Pinia、Vue Router、Axios、ECharts
- 后端：Node.js、NestJS、TypeScript、Prisma、PostgreSQL、Redis、JWT
- 基础设施：TimescaleDB、EMQX、Docker Compose

## 目录结构

```text
weikong-iot-platform/
  apps/
    web/        # Vue 前端
    api/        # NestJS API
    ingest/     # MQTT 接入服务占位
    consumer/   # 事件消费服务占位
  packages/
    shared/     # 共享包占位
  docker-compose.yml
  .env.example
```

## 本地启动

先启动基础设施：

```bash
docker compose up -d postgres redis timescaledb emqx
```

启动后端：

```bash
cd apps/api
npm install
copy .env.example .env
npx prisma migrate dev --name init
npx prisma db seed
npm run start:dev
```

启动前端：

```bash
cd apps/web
npm install
npm run dev
```

浏览器访问 `http://localhost:5173`，API 默认运行在 `http://localhost:3100`。

## 默认账号

执行种子脚本后可使用：

```text
用户名：admin
密码：admin123456
```

## Docker Compose 启动

构建并启动全部服务：

```bash
docker compose up -d --build
```

容器方式首次启动后，初始化数据库：

```bash
docker compose exec api npx prisma migrate deploy
docker compose exec api npx prisma db seed
```

## 数据库迁移

开发环境创建迁移：

```bash
cd apps/api
npx prisma migrate dev --name init
```

应用已有迁移：

```bash
npx prisma migrate deploy
```

## 权限说明

- `SUPER_ADMIN`：可访问全部用户与租户管理接口。
- `TENANT_ADMIN`：只能管理本租户的普通用户。
- `TENANT_USER`：不能访问用户管理和租户管理接口。

注册账号默认创建一个新租户，并将注册用户设置为该租户的 `TENANT_ADMIN`。

## 常见问题

### PowerShell 无法执行 npm

若系统限制执行 `npm.ps1`，请使用 `npm.cmd` 和 `npx.cmd`。

### 前端无法请求后端

确认 API 已运行在 `http://localhost:3100`。如需调整地址，设置前端环境变量 `VITE_API_BASE_URL`。

### 登录提示账号或密码错误

确认已经执行 `npx prisma db seed`。默认账号只会在种子脚本执行后创建。

### 重置密码验证码

V1.0 使用模拟验证码 `123456`。

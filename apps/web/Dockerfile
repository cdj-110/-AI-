FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
ARG PORTAL_MODE=tenant
ARG VITE_API_BASE_URL=
ARG VITE_GOVIEW_DESIGNER_URL=
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_GOVIEW_DESIGNER_URL=$VITE_GOVIEW_DESIGNER_URL
RUN npm run build:${PORTAL_MODE} && cp -r dist/${PORTAL_MODE} /app/release

FROM nginx:alpine
COPY --from=builder /app/release /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80

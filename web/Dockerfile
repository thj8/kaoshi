# 构建阶段：API/WS 地址留空 = 使用相对路径（经 nginx 反代，任意 IP/域名访问均可）
ARG VITE_API_BASE=""
ARG VITE_WS_BASE=""

FROM node:22-alpine AS builder
WORKDIR /app
ARG VITE_API_BASE
ARG VITE_WS_BASE
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_WS_BASE=$VITE_WS_BASE
COPY package.json package-lock.json .npmrc ./
# 可选：内网 registry（docker compose build --build-arg NPM_REGISTRY=http://...）
ARG NPM_REGISTRY=""
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi && npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80

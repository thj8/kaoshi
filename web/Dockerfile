# 构建阶段注入 API 地址（容器内 nginx 代理或浏览器直连）
ARG VITE_API_BASE=""
ARG VITE_WS_BASE=""

FROM node:22-alpine AS builder
WORKDIR /app
ARG VITE_API_BASE
ARG VITE_WS_BASE
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_WS_BASE=$VITE_WS_BASE
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80

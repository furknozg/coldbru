# syntax=docker/dockerfile:1
# Web image (D-085): build the Vue app, serve it with nginx and route /api/<service>/ to the Go services.
# Build context is web/ (docker-compose.yml). Nothing outside web/ is needed to build.

# ---- build ----------------------------------------------------------------
FROM node:24.22.0-alpine AS build
# package.json has no "packageManager" field, so pin pnpm here (same major as the lockfile / local toolchain).
ARG PNPM_VERSION=12.8.1
RUN corepack enable && corepack prepare "pnpm@${PNPM_VERSION}" --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
# `pnpm build` = vue-tsc type-check + vite build, so a type error fails the image build.
RUN pnpm build

# ---- serve ----------------------------------------------------------------
FROM nginx:1.30.5-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80

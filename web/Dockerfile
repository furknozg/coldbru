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
# The nginx image's entrypoint renders /etc/nginx/templates/*.template into conf.d at start with
# envsubst. NGINX_ENVSUBST_FILTER limits it to variables named CAMPAIGN_*, so nginx's own $variables are
# left alone. CAMPAIGN_CTA_URL is where /r/ falls back to when the redirect service fails (D-106);
# docker-compose.yml passes the stack's value, the default below keeps a standalone image working.
ENV NGINX_ENVSUBST_FILTER=CAMPAIGN_ \
    CAMPAIGN_CTA_URL=https://shop.example.com
COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80

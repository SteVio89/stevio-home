FROM node:25-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npx vitest run
RUN npm test
RUN npm audit
RUN npm run build

FROM nginx:alpine

RUN apk add --no-cache curl

# Create stevio user with same UID/GID as backend so nginx can read shared assets
RUN addgroup -g 1001 -S stevio && adduser -u 1001 -S -G stevio stevio

# nginx needs write access to these dirs when running as non-root worker
RUN chown -R stevio:stevio /var/cache/nginx /var/log/nginx \
    && touch /var/run/nginx.pid && chown stevio:stevio /var/run/nginx.pid

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
# Not under conf.d/ (which nginx auto-includes as server blocks) — this snippet
# is pulled in explicitly via `include` from nginx.conf where headers are needed.
COPY nginx-security-headers.conf /etc/nginx/security-headers.conf

# Set nginx worker to run as stevio (must be in main config, not server block)
RUN sed -i 's/^user  nginx;/user  stevio;/' /etc/nginx/nginx.conf

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD curl -f http://localhost/ || exit 1

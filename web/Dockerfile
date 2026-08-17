FROM node:20.19.5-bookworm-slim@sha256:9e70124bd00f47dd023e349cd587132ae61892acc0e47ed641416c3e18f401c3 AS build

WORKDIR /src/web

COPY web/.npmrc web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

RUN printf '%s\n' \
    'worker_processes auto;' \
    'pid /tmp/nginx.pid;' \
    'error_log /dev/stderr notice;' \
    'events { worker_connections 1024; }' \
    'http {' \
    '  include /etc/nginx/mime.types;' \
    '  default_type application/octet-stream;' \
    '  access_log /dev/stdout;' \
    '  sendfile on;' \
    '  client_body_temp_path /tmp/client_body;' \
    '  proxy_temp_path /tmp/proxy;' \
    '  fastcgi_temp_path /tmp/fastcgi;' \
    '  uwsgi_temp_path /tmp/uwsgi;' \
    '  scgi_temp_path /tmp/scgi;' \
    '  server {' \
    '    listen 8080;' \
    '    server_name _;' \
    '    root /usr/share/nginx/html;' \
    '    index index.html;' \
    '    location / {' \
    '      try_files $uri $uri/ /index.html;' \
    '    }' \
    '  }' \
    '}' > /tmp/nginx.conf

FROM nginx:1.30.4-alpine3.24-slim@sha256:ddde39c6e51f02fde7410c2e9c234cf2d0a4c7bdbbe176aeb37d8ad7ab4eb58c

COPY --from=build /tmp/nginx.conf /etc/nginx/nginx.conf
COPY --from=build /src/web/dist/ /usr/share/nginx/html/

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]

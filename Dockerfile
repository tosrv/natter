# syntax=docker/dockerfile:1
FROM node:22 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
# Optional build secret "extra_ca": extra CA bundle for build-time egress
# through a TLS-intercepting proxy. Empty/absent in normal builds.
RUN --mount=type=secret,id=extra_ca \
    if [ -s /run/secrets/extra_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/extra_ca; fi; \
    npm ci
COPY web/ ./
RUN npm run build

FROM node:22 AS widget
WORKDIR /src/widget
COPY widget/package.json widget/package-lock.json ./
RUN --mount=type=secret,id=extra_ca \
    if [ -s /run/secrets/extra_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/extra_ca; fi; \
    npm ci
COPY widget/ ./
RUN npm run build

FROM golang:1.24 AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=extra_ca \
    if [ -s /run/secrets/extra_ca ]; then cat /etc/ssl/certs/ca-certificates.crt /run/secrets/extra_ca > /tmp/ca.pem; export SSL_CERT_FILE=/tmp/ca.pem; fi; \
    go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:bookworm-slim
# postgresql-client-16 dari PGDG (apt.postgresql.org) agar pg_dump cocok dengan server 16.
# Secret opsional extra_ca dipasang sementara untuk curl (proxy TLS-intercepting), lalu dihapus.
RUN --mount=type=secret,id=extra_ca \
    set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg; \
    if [ -s /run/secrets/extra_ca ]; then cp /run/secrets/extra_ca /usr/local/share/ca-certificates/extra_ca.crt; update-ca-certificates >/dev/null; fi; \
    install -d /usr/share/postgresql-common/pgdg; \
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg; \
    echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list; \
    apt-get update; \
    mkdir -p /usr/share/man/man1; \
    apt-get install -y --no-install-recommends postgresql-client-16; \
    apt-get purge -y --auto-remove gnupg; \
    if [ -s /run/secrets/extra_ca ]; then \
      rm -f /usr/local/share/ca-certificates/extra_ca.crt; \
      update-ca-certificates --fresh >/dev/null; \
    fi; \
    rm -rf /var/lib/apt/lists/*; \
    groupadd --system --gid 65532 nonroot; \
    useradd --system --uid 65532 --gid 65532 --create-home --home-dir /home/nonroot nonroot
WORKDIR /app
COPY --from=go /out/server /app/server
COPY --from=web /src/web/dist /app/web
COPY --from=widget /src/widget/dist/widget.js /app/web/widget.js
COPY --chmod=0755 deploy/*.sh /app/deploy/
ENV STATIC_DIR=/app/web LISTEN_ADDR=:8080 BACKUP_DIR=/backups
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]

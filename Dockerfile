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

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=go /out/server /app/server
COPY --from=web /src/web/dist /app/web
ENV STATIC_DIR=/app/web LISTEN_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]

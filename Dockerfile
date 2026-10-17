# syntax=docker/dockerfile:1
FROM node:24-bookworm-slim AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=secret,id=proxy_ca \
    if [ -s /run/secrets/proxy_ca ]; then \
      NODE_EXTRA_CA_CERTS=/run/secrets/proxy_ca npm ci --strict-ssl=true; \
    else npm ci --strict-ssl=true; fi
COPY web/ ./
RUN npm run build

FROM golang:1.27-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN GOPROXY=off CGO_ENABLED=1 go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/oxnet-work ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
RUN mkdir -p /data /app/web/dist && chown -R 10001:10001 /data
COPY --from=go-build /out/oxnet-work /app/oxnet-work
COPY --from=web-build /src/web/dist/ /app/web/dist/
USER 10001:10001
ENV OXNET_DATA_DIR=/data OXNET_WEB_DIR=/app/web/dist OXNET_LISTEN=0.0.0.0:8080 OXNET_ALLOW_UNAUTHENTICATED_BIND=true
EXPOSE 8080
ENTRYPOINT ["/app/oxnet-work"]

FROM node:24-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# The Go server owns the same-origin API in the packaged application.
RUN rm -rf app/api && NEXT_STATIC_EXPORT=true npm run build

FROM golang:1.23-alpine AS server-build
WORKDIR /src/worker
COPY worker/go.mod worker/go.sum ./
RUN go mod download
COPY worker/ ./
COPY --from=web-build /src/web/out ./cmd/server/web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cdc-lite-server ./cmd/server

FROM alpine:3.21
RUN addgroup -S cdc && adduser -S -G cdc -h /app cdc && mkdir /data && chown -R cdc:cdc /app /data
WORKDIR /app
COPY --from=server-build /out/cdc-lite-server /usr/local/bin/cdc-lite-server
USER cdc
ENV PORT=3000 METADATA_DB_PATH=/data/cdc-lite.db
EXPOSE 3000
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 CMD wget -q -O - http://127.0.0.1:3000/healthz || exit 1
ENTRYPOINT ["cdc-lite-server"]

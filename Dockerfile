# Build stage for Go backend
FROM golang:1.25-alpine AS go-builder

WORKDIR /usr/src/app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Build stage for frontend
FROM oven/bun:1-alpine AS web-builder

WORKDIR /usr/src/app

COPY app/package.json app/bun.lock* ./
RUN bun install --frozen-lockfile

COPY app/ .
ARG VITE_SERVER_API_PORT=8080
ENV VITE_SERVER_API_PORT=${VITE_SERVER_API_PORT}
RUN bun run build

# Runtime stage
FROM alpine:3.19

WORKDIR /usr/src/app

RUN apk add --no-cache ca-certificates wget && \
    gosuArch=$(apk --print-arch | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/') && \
    wget -O /usr/local/bin/gosu "https://github.com/tianon/gosu/releases/download/1.19/gosu-$gosuArch" && \
    chmod +x /usr/local/bin/gosu

RUN adduser -D -s /bin/sh appuser

COPY --from=go-builder /usr/src/app/server /usr/local/bin/server
RUN chmod +x /usr/local/bin/server && chown appuser:appuser /usr/local/bin/server
COPY --from=web-builder /usr/src/app/dist ./dist

RUN mkdir -p /data/unblink && chown -R appuser:appuser /data/unblink

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV PORT=8080
ENV CONFIG_DIR=/data/unblink
ENV DIST_PATH=/usr/src/app/dist
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/usr/local/bin/server"]

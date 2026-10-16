FROM --platform=$BUILDPLATFORM golang:1.26.9-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /enterprise ./cmd/enterprise

FROM alpine:3.22
RUN apk add --no-cache ca-certificates su-exec && adduser -D -u 10001 soren && mkdir /data && chown soren:soren /data
COPY --from=build /enterprise /usr/local/bin/soren-enterprise
COPY --chmod=755 deploy/enterprise/entrypoint.sh /usr/local/bin/soren-entrypoint
# 入口仅初始化新卷目录，业务进程由 su-exec 降权为 10001:10001。
ENV ENTERPRISE_LISTEN=0.0.0.0:8091 SOREN_DATA_DIR=/data GIN_MODE=release
EXPOSE 8091
ENTRYPOINT ["/usr/local/bin/soren-entrypoint", "/usr/local/bin/soren-enterprise"]

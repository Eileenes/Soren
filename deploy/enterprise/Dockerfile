FROM golang:1.26.9-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /enterprise ./cmd/enterprise

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 soren && mkdir /data && chown soren:soren /data
COPY --from=build /enterprise /usr/local/bin/soren-enterprise
USER soren
ENV ENTERPRISE_LISTEN=0.0.0.0:8091 SOREN_DATA_DIR=/data GIN_MODE=release
EXPOSE 8091
ENTRYPOINT ["/usr/local/bin/soren-enterprise"]

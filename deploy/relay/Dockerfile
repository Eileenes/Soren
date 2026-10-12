FROM golang:1.26.9-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /relay ./cmd/relay
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 soren
COPY --from=build /relay /usr/local/bin/soren-relay
USER soren
ENV RELAY_LISTEN=0.0.0.0:8090
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/soren-relay"]

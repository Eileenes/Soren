FROM golang:1.26.9-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /templates ./cmd/templates

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 soren && mkdir /data && chown soren:soren /data
COPY --from=build /templates /usr/local/bin/soren-templates
USER soren
ENV TEMPLATE_LISTEN=0.0.0.0:8090 TEMPLATE_DATABASE=/data/templates.db TEMPLATE_PUBLISHERS_FILE=/config/publishers.json
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/soren-templates"]

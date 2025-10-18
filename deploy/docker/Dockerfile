# syntax=docker/dockerfile:1.5

FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/openmcpd ./cmd/openmcpd

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata bash gettext
COPY --from=builder /out/openmcpd /app/openmcpd
COPY deploy/docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV OPENMCP_CONFIG="/etc/openmcp/config/openmcp.json"
ENV OPENMCP_DATA_DIR="/var/lib/openmcp"
VOLUME ["/var/lib/openmcp"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/app/openmcpd"]

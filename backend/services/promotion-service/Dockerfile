FROM golang:1.25-alpine AS builder

WORKDIR /app/services/promotion-service

# Install dependencies for building
RUN apk add --no-cache git tzdata

# Copy the shared pkg and the service code (build context is backend/)
COPY pkg/ /app/pkg/
COPY services/promotion-service/ /app/services/promotion-service/

# Build the binary using local module resolution (ignores go.work)
ENV GOWORK=off
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /promotion-service ./main.go

# Final stage
FROM alpine:3.19

WORKDIR /root/

# Copy timezone data and certs
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy built binary
COPY --from=builder /promotion-service .

ENV PORT=8080
EXPOSE 8080

CMD ["./promotion-service"]

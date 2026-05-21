FROM golang:1.25-alpine AS builder

WORKDIR /app/services/auth-service

# Install dependencies for building
RUN apk add --no-cache git tzdata

# Copy the shared pkg and the service code (build context is backend/)
COPY pkg/ /app/pkg/
COPY services/auth-service/ /app/services/auth-service/

# Build the binary using local module resolution (ignores go.work)
ENV GOWORK=off
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /auth-service ./main.go

# Final stage
FROM alpine:3.19

WORKDIR /root/

# Copy timezone data and certs
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy built binary
COPY --from=builder /auth-service .

ENV PORT=8080
EXPOSE 8080

CMD ["./auth-service"]

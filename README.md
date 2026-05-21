# Clay Platform

Monorepo for Clay — a microservices-based on-demand service platform (Gojek-style).

## Structure

```
clay-app/
├── backend/
│   ├── go.work              # Go workspace
│   ├── services/            # 24 microservices
│   │   ├── gateway/         # API Gateway
│   │   ├── auth-service/
│   │   ├── user-service/
│   │   ├── payment-service/
│   │   ├── food-order-service/
│   │   ├── delivery-order-service/
│   │   ├── ride-order-service/
│   │   ├── chat-service/
│   │   ├── notification-service/
│   │   ├── push-service/
│   │   ├── sms-service/
│   │   ├── email-service/
│   │   ├── search-service/
│   │   ├── geo-service/
│   │   ├── matching-service/
│   │   ├── merchant-service/
│   │   ├── rating-service/
│   │   ├── promotion-service/
│   │   ├── pricing-service/
│   │   ├── wallet-service/
│   │   ├── history-service/
│   │   ├── tracking-service/
│   │   ├── audit-log-service/
│   │   └── security-service/
│   ├── pkg/                 # Shared libraries
│   └── infra/               # Docker, K8s, Terraform
├── docs/                    # API specs, docs
├── proto/                   # Protobuf definitions
├── scripts/                 # Personal scripts (gitignored)
├── .github/workflows/       # CI/CD pipelines
├── Jenkinsfile              # Single CI/CD pipeline
└── README.md
```

## Prerequisites

- Go 1.25+
- Docker
- Kubernetes (kind/minikube for local)
- kubectl

## Getting Started

```bash
# Clone
git clone <repo-url> clay-app
cd clay-app/backend

# Sync Go workspace
go work sync

# Run a service locally
cd services/auth-service
go run .
```

## CI/CD

Single `Jenkinsfile` at root with conditional execution — only changed services are rebuilt and deployed. Runs on Windows agents (`bat` commands).

## Services

| Service | Port | Description |
|---|---|---|
| gateway | 8080 | API Gateway (LoadBalancer) |
| auth-service | 8081 | Authentication & Authorization |
| user-service | 8082 | User management |
| payment-service | 8083 | Payment processing |
| food-order-service | 8084 | Food ordering |
| delivery-order-service | 8085 | Delivery management |
| ride-order-service | 8086 | Ride hailing |
| chat-service | 8087 | Real-time messaging |
| notification-service | 8088 | Notifications |
| push-service | 8089 | Push notifications |
| sms-service | 8090 | SMS gateway |
| email-service | 8091 | Email service |
| search-service | 8092 | Search & discovery |
| geo-service | 8093 | Geospatial queries |
| matching-service | 8094 | Driver/rider matching |
| merchant-service | 8095 | Merchant management |
| rating-service | 8096 | Ratings & reviews |
| promotion-service | 8097 | Promotions & vouchers |
| pricing-service | 8098 | Dynamic pricing |
| wallet-service | 8099 | Digital wallet |
| history-service | 8100 | Order history |
| tracking-service | 8101 | Real-time tracking |
| audit-log-service | 8102 | Audit logging |
| security-service | 8103 | Security & compliance |

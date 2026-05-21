# Clay Infrastructure

## Quick Start (Local Development)

```bash
# Start all services
cd clay-infra
docker-compose up -d

# Wait for infra to be ready (~30s)
docker-compose ps

# Stop everything
docker-compose down -v
```

## Service URLs (Local)

| Service | URL |
|---------|-----|
| API Gateway | http://localhost:8080 |
| Auth | http://localhost:8001 |
| User | http://localhost:8002 |
| Email | http://localhost:8003 |
| Payment | http://localhost:8004 |
| Notification | http://localhost:8005 |
| Tracking | http://localhost:8006 |
| Audit Log | http://localhost:8007 |
| Chat | http://localhost:8008 |
| Geo | http://localhost:8009 |
| Matching | http://localhost:8010 |
| Merchant | http://localhost:8011 |
| Pricing | http://localhost:8012 |
| Promotion | http://localhost:8013 |
| Push | http://localhost:8014 |
| Rating | http://localhost:8015 |
| Ride Order | http://localhost:8016 |
| Food Order | http://localhost:8017 |
| Delivery Order | http://localhost:8018 |
| Search | http://localhost:8019 |
| Security | http://localhost:8020 |
| SMS | http://localhost:8021 |
| Wallet | http://localhost:8022 |
| History | http://localhost:8023 |

## Tooling

| Tool | URL |
|------|-----|
| Kafka UI | http://localhost:9000 |
| Elasticsearch | http://localhost:9200 |
| Adminer (Auth) | http://localhost:9001 |
| Adminer (User) | http://localhost:9002 |

## Kubernetes

```bash
kubectl apply -f k8s/base/
kubectl apply -f k8s/infra/
kubectl apply -f k8s/services/
```


# Port Mapping Standard - Clay Microservices

Dokumen ini berisi standar alokasi port untuk seluruh microservices di proyek Clay. Harap ikuti standar ini saat membuat `docker-compose.yml` atau melakukan konfigurasi `main.go` agar tidak terjadi bentrok port saat dijalankan bersamaan.

## 🚀 Master Port Mapping

| #  | Service Name                |    App Port    | Postgres | Redis |     MongoDB     |  Others  | Adminer |
| -- | :-------------------------- | :------------: | :------: | :---: | :-------------: | :------: | :-----: |
| 1  | **clay-gateway**      | **8080** |    -    | 6370 |        -        |    -    |    -    |
| 2  | clay-auth-service           |      8001      |   5431   | 6371 |        -        |    -    |  9001  |
| 3  | clay-user-service           |      8002      |   5432   | 6372 |        -        |    -    |  9002  |
| 4  | clay-email-service          |      8003      |   5433   | 6373 |        -        |    -    |  9003  |
| 5  | clay-payment-service        |      8004      |   5434   | 6374 |        -        |    -    |  9004  |
| 6  | clay-notification-service   |      8005      |   5435   |   -   |        -        |    -    |  9005  |
| 7  | clay-tracking-service       |      8006      |    -    | 6376 | **27019** |    -    |    -    |
| 8  | clay-audit-log-service      |      8007      |    -    |   -   | **27018** |    -    |    -    |
| 9  | clay-chat-service           |      8008      |    -    |   -   | **27017** |    -    |    -    |
| 10 | clay-geo-service            |      8009      |   5439   | 6379 |        -        |    -    |  9009  |
| 11 | clay-matching-service       |      8010      |    -    | 6380 |        -        |    -    |    -    |
| 12 | clay-merchant-service       |      8011      |   5441   |   -   | **27020** |    -    |  9011  |
| 13 | clay-pricing-service        |      8012      |   5442   | 6382 |        -        |    -    |  9012  |
| 14 | clay-promotion-service      |      8013      |   5443   | 6383 |        -        |    -    |  9013  |
| 15 | clay-push-service           |      8014      |    -    |   -   |        -        |    -    |    -    |
| 16 | clay-rating-service         |      8015      |   5445   | 6385 |        -        |    -    |  9015  |
| 17 | clay-ride-order-service     |      8016      |   5446   | 6386 |        -        |    -    |  9016  |
| 18 | clay-food-order-service     |      8017      |   5447   | 6387 | **27021** |    -    |  9017  |
| 19 | clay-delivery-order-service |      8018      |   5448   | 6388 |        -        |    -    |  9018  |
| 20 | clay-search-service         |      8019      |    -    | 6389 |        -        | ES: 9200 |    -    |
| 21 | clay-security-service       |      8020      |   5450   | 6390 |        -        |    -    |  9020  |
| 22 | clay-sms-service            |      8021      |   5451   | 6391 |        -        |    -    |  9021  |
| 23 | clay-wallet-service         |      8022      |   5452   | 6392 |        -        |    -    |  9022  |
| 24 | clay-history-service        |      8023      |   5453   | 6393 |        -        |    -    |  9023  |

> **Adminer** hanya untuk service yang punya PostgreSQL. Service MongoDB diakses via **MongoDB Compass** (desktop app, tidak butuh container tambahan).

---

## 🟠 Kafka Infrastructure (Shared)

Kafka adalah shared infrastructure — satu cluster dipakai semua service. Tidak perlu port per-service.

| Component                         | Host Port | Internal Port | Keterangan                           |
| --------------------------------- | :-------: | :-----------: | ------------------------------------ |
| **Zookeeper**               |   2181   |     2181     | Kafka cluster coordinator            |
| **Kafka Broker**            |   9092   |     9092     | Producer & consumer endpoint         |
| **Kafka Broker** (external) |   29092   |     29092     | Untuk akses dari luar Docker network |
| **Kafka UI**                |   9000   |     8080     | Web UI monitoring topics & consumers |

### Topic Naming Convention

```
{domain}.{event}
# contoh:
auth.login_success
order.created
payment.charged
driver.location_updated
```

### Siapa yang produce & consume Kafka

| Producer                    | Topic                                     | Consumer                                                                                       |
| --------------------------- | ----------------------------------------- | ---------------------------------------------------------------------------------------------- |
| clay-auth-service           | `auth.*`                                | clay-audit-log-service                                                                         |
| clay-ride-order-service     | `order.created`, `order.completed`    | clay-matching-service, clay-audit-log-service, clay-history-service, clay-notification-service |
| clay-food-order-service     | `food_order.*`                          | clay-audit-log-service, clay-history-service, clay-notification-service                        |
| clay-delivery-order-service | `delivery_order.*`                      | clay-audit-log-service, clay-history-service, clay-notification-service                        |
| clay-payment-service        | `payment.*`                             | clay-wallet-service, clay-audit-log-service, clay-notification-service                         |
| clay-matching-service       | `driver.matched`, `driver.dispatched` | clay-ride-order-service, clay-notification-service                                             |
| clay-geo-service            | `driver.location_updated`               | clay-tracking-service, clay-matching-service                                                   |
| clay-security-service       | `security.user_flagged`                 | clay-audit-log-service, clay-notification-service                                              |

---

## 🛠️ Panduan Implementasi

### 1. Docker Compose — PostgreSQL

```yaml
services:
  postgres-payment:
    image: postgres:16-alpine
    ports:
      - "5434:5432"
  adminer:
    image: adminer
    ports:
      - "9004:8080"
```

### 2. Docker Compose — Redis

```yaml
services:
  redis-auth:
    image: redis:7-alpine
    ports:
      - "6371:6379"
```

### 3. Docker Compose — MongoDB

MongoDB digunakan oleh 5 service: **chat**, **tracking**, **audit-log**, **merchant**, **food-order**.
Masing-masing punya instance sendiri dengan port berbeda agar tetap terisolasi (database-per-service principle).

```yaml
services:
  mongo-chat:
    image: mongo:7
    ports:
      - "27017:27017"
  mongo-audit:
    image: mongo:7
    ports:
      - "27018:27017"
  mongo-tracking:
    image: mongo:7
    ports:
      - "27019:27017"
  mongo-merchant:
    image: mongo:7
    ports:
      - "27020:27017"
  mongo-food:
    image: mongo:7
    ports:
      - "27021:27017"
```

**Cara akses (development):** Gunakan **MongoDB Compass** (desktop app). Tambahkan saved connections:

| Connection Name | URI |
|---|---|
| clay-chat | `mongodb://localhost:27017` |
| clay-audit-log | `mongodb://localhost:27018` |
| clay-tracking | `mongodb://localhost:27019` |
| clay-merchant | `mongodb://localhost:27020` |
| clay-food-order | `mongodb://localhost:27021` |

Tinggal `docker-compose up -d` → buka Compass → klik saved connection sesuai service yang ingin diinspect. Tidak perlu container GUI tambahan.

### 4. Docker Compose — Elasticsearch

```yaml
services:
  elasticsearch:
    image: elasticsearch:8.13.0
    ports:
      - "9200:9200"
      - "9300:9300"  # cluster transport
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
```

### 5. Docker Compose — Kafka

```yaml
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.6.0
    ports:
      - "2181:2181"
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

  kafka:
    image: confluentinc/cp-kafka:7.6.0
    ports:
      - "9092:9092"
      - "29092:29092"
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_LISTENERS: INTERNAL://0.0.0.0:9092,EXTERNAL://0.0.0.0:29092
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka:9092,EXTERNAL://localhost:29092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: INTERNAL

  kafka-ui:
    image: provectuslabs/kafka-ui:latest
    ports:
      - "9000:8080"
    environment:
      KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:9092
```

### 6. Environment Variables

```env
PORT=8001
DB_PORT=5431
REDIS_PORT=6371
KAFKA_BROKERS=kafka:9092
```

---

## 📊 Ringkasan Database per Service

| Database                | Services                                                                                                                                                                      |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **PostgreSQL**    | auth, user, email, payment, notification, geo, merchant, pricing, promotion, rating, ride-order, food-order, delivery-order, security, sms, wallet, history (17 services)     |
| **Redis**         | gateway, auth, user, email, payment, tracking, geo, matching, pricing, promotion, rating, ride-order, food-order, delivery-order, search, security, sms, wallet (18 services) |
| **MongoDB**       | chat, tracking, audit-log, merchant, food-order (5 services)                                                                                                                  |
| **Elasticsearch** | search (1 service)                                                                                                                                                            |
| **Kafka**         | semua service yang punya event-driven flow (shared)                                                                                                                           |

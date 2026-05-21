# Clay Documentation

Selamat datang di dokumentasi proyek Clay — superapp platform dengan 23 microservices.

## Daftar Dokumen

| Dokumen | Keterangan |
|---------|------------|
| [Port Mapping Standard](PORT_MAPPING.md) | Alokasi port tiap microservice |
| [ERD Design](clay_erd.excalidraw) | Rancangan database semua service |
| [Architecture Design](microservice_architecture_final.excalidraw) | Rancangan arsitektur microservice |
| [Postman Collection](clay-postman-collection.json) | Koleksi API endpoint (238 request) |

---

## Ringkasan API Endpoint

Semua request melewati **API Gateway** (`http://localhost:8080`).  
Total: **238 endpoint** di 23 service.

| # | Service | Endpoint | Port Langsung |
|---|---------|:--------:|:-------------:|
| 1 | Auth Service | 14 | 8001 |
| 2 | User Service | 23 | 8002 |
| 3 | Ride Order Service | 11 | 8016 |
| 4 | Food Order Service | 18 | 8017 |
| 5 | Delivery Order Service | 11 | 8018 |
| 6 | Merchant Service | 22 | 8011 |
| 7 | Payment Service | 10 | 8004 |
| 8 | Wallet Service | 10 | 8022 |
| 9 | Promotion Service | 3 | 8013 |
| 10 | Rating Service | 5 | 8015 |
| 11 | Chat Service | 8 | 8008 |
| 12 | Search Service | 5 | 8019 |
| 13 | Geo Service | 14 | 8009 |
| 14 | Pricing Service | 4 | 8012 |
| 15 | Tracking Service | 4 | 8006 |
| 16 | Notification Service | 7 | 8005 |
| 17 | Matching Service | 8 | 8010 |
| 18 | History Service | 9 | 8023 |
| 19 | Security Service | 10 | 8020 |
| 20 | Audit Log Service | 7 | 8007 |
| 21 | Webhooks | 2 | — |
| 22 | Internal (S2S) | 9 | — |
| 23 | Health Checks | 24 | — |
| | **Total** | **238** | |

---

## Endpoint per Service

### 01 · Auth Service (14 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/auth/register` | Public |
| POST | `/auth/request-otp` | Public |
| POST | `/auth/verify-otp` | Public |
| POST | `/auth/login` | Public |
| POST | `/auth/login/otp` | Public |
| POST | `/auth/refresh-token` | Public |
| POST | `/auth/logout` | JWT |
| POST | `/auth/logout-all` | JWT |
| GET | `/auth/sessions` | JWT |
| POST | `/auth/sessions/revoke-all` | JWT |
| DELETE | `/auth/sessions/{sessionId}` | JWT |
| POST | `/auth/password/forgot` | Public |
| POST | `/auth/password/reset` | Public |
| PUT | `/auth/password/change` | JWT |

### 02 · User Service (23 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET / POST / PUT | `/users/me` | JWT |
| PUT | `/users/me/avatar` | JWT |
| POST | `/users/me/referral/apply` | JWT |
| GET | `/users/{userId}` | JWT |
| GET / POST | `/addresses` | JWT |
| PUT / DELETE | `/addresses/{addressId}` | JWT |
| PUT | `/addresses/{addressId}/default` | JWT |
| GET / PUT | `/settings` | JWT |
| POST | `/drivers/register` | JWT |
| GET / PUT | `/drivers/me` | JWT (driver) |
| GET / POST | `/drivers/me/documents` | JWT (driver) |
| GET / DELETE | `/drivers/me/documents/{documentId}` | JWT (driver) |
| GET | `/drivers/{driverId}` | JWT |
| PUT | `/drivers/{driverId}/status` | JWT (driver/admin) |
| GET | `/drivers/{driverId}/verification` | JWT (driver/admin) |

### 03 · Ride Order Service (11 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/ride/orders/estimate` | JWT (user) |
| POST | `/ride/orders` | JWT (user) |
| GET | `/ride/orders/active` | JWT (user) |
| GET | `/ride/orders/history` | JWT (user) |
| GET | `/ride/orders/{orderId}` | JWT |
| POST | `/ride/orders/{orderId}/cancel` | JWT (user) |
| GET | `/ride/orders/{orderId}/fare-breakdown` | JWT |
| POST | `/ride/orders/{orderId}/rate` | JWT (user) |
| POST | `/ride/driver/orders/{orderId}/accept` | JWT (driver) |
| POST | `/ride/driver/orders/{orderId}/reject` | JWT (driver) |
| PUT | `/ride/driver/orders/{orderId}/status` | JWT (driver) |

### 04 · Food Order Service (18 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/food/orders/estimate` | JWT (user) |
| POST | `/food/orders` | JWT (user) |
| GET | `/food/orders/active` | JWT (user) |
| GET | `/food/orders/history` | JWT (user) |
| GET | `/food/orders/{orderId}` | JWT |
| POST | `/food/orders/{orderId}/cancel` | JWT (user) |
| GET | `/food/orders/{orderId}/fare-breakdown` | JWT |
| POST | `/food/orders/{orderId}/rate` | JWT (user) |
| GET | `/food/merchant/orders` | JWT (merchant) |
| GET | `/food/merchant/orders/{orderId}` | JWT (merchant) |
| POST | `/food/merchant/orders/{orderId}/confirm` | JWT (merchant) |
| POST | `/food/merchant/orders/{orderId}/reject` | JWT (merchant) |
| PUT | `/food/merchant/orders/{orderId}/status` | JWT (merchant) |
| POST | `/food/driver/orders/{orderId}/accept` | JWT (driver) |
| POST | `/food/driver/orders/{orderId}/reject` | JWT (driver) |
| PUT | `/food/driver/orders/{orderId}/status` | JWT (driver) |
| POST | `/food/driver/orders/{orderId}/pickup` | JWT (driver) |
| POST | `/food/driver/orders/{orderId}/deliver` | JWT (driver) |

### 05 · Delivery Order Service (11 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/delivery/orders/estimate` | JWT (user) |
| POST | `/delivery/orders` | JWT (user) |
| GET | `/delivery/orders/active` | JWT (user) |
| GET | `/delivery/orders/history` | JWT (user) |
| GET | `/delivery/orders/{orderId}` | JWT |
| POST | `/delivery/orders/{orderId}/cancel` | JWT (user) |
| GET | `/delivery/orders/{orderId}/fare-breakdown` | JWT |
| POST | `/delivery/orders/{orderId}/rate` | JWT (user) |
| POST | `/delivery/driver/orders/{orderId}/accept` | JWT (driver) |
| POST | `/delivery/driver/orders/{orderId}/reject` | JWT (driver) |
| PUT | `/delivery/driver/orders/{orderId}/status` | JWT (driver) |

### 06 · Merchant Service (22 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/merchants` | JWT |
| GET / PUT | `/merchants/me` | JWT (merchant) |
| GET | `/merchants/{merchantId}` | JWT |
| PATCH | `/merchants/{merchantId}/status` | JWT (merchant) |
| GET / PUT | `/merchants/{merchantId}/operating-hours` | JWT |
| GET / POST | `/merchants/{merchantId}/bank-accounts` | JWT (merchant) |
| DELETE | `/merchants/{merchantId}/bank-accounts/{accountId}` | JWT (merchant) |
| PATCH | `/merchants/{merchantId}/bank-accounts/{accountId}/set-primary` | JWT (merchant) |
| GET / POST | `/merchants/{merchantId}/menu/categories` | JWT |
| PATCH | `/merchants/{merchantId}/menu/categories/reorder` | JWT (merchant) |
| PUT / DELETE | `/merchants/{merchantId}/menu/categories/{categoryId}` | JWT (merchant) |
| GET / POST | `/merchants/{merchantId}/menu/items` | JWT |
| GET / PUT / DELETE | `/merchants/{merchantId}/menu/items/{itemId}` | JWT |
| PATCH | `/merchants/{merchantId}/menu/items/{itemId}/availability` | JWT (merchant) |

### 07 · Payment Service (10 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET / POST | `/payment-methods` | JWT |
| DELETE | `/payment-methods/{methodId}` | JWT |
| POST | `/payment-methods/{methodId}/set-default` | JWT |
| GET | `/transactions` | JWT |
| GET | `/transactions/{transactionId}` | JWT |
| POST | `/cod/verify/initiate` | JWT (user) |
| GET | `/cod/verify/{verificationId}/status` | JWT |
| POST | `/cod/verify/{verificationId}/otp` | JWT |
| POST | `/cod/verify/{verificationId}/respond` | JWT |

### 08 · Wallet Service (10 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/wallet` | JWT |
| POST | `/wallet/topup` | JWT |
| POST | `/wallet/topup/callback` | Public (webhook) |
| POST | `/wallet/transfer` | JWT |
| GET | `/wallet/transactions` | JWT |
| GET | `/wallet/transactions/{txId}` | JWT |
| GET | `/driver/wallet/balance` | JWT (driver) |
| GET | `/driver/wallet/transactions` | JWT (driver) |
| GET | `/driver/settlements` | JWT (driver) |
| GET | `/driver/settlements/{orderId}` | JWT (driver) |

### 09 · Promotion Service (3 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/promos/validate` | JWT |
| GET | `/vouchers` | JWT |
| POST | `/vouchers/claim` | JWT |

### 10 · Rating Service (5 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/ratings` | JWT |
| GET | `/ratings/me/given` | JWT |
| GET | `/ratings/me/received` | JWT |
| GET | `/ratings/orders/{orderId}` | JWT |
| GET | `/ratings/{subjectType}/{subjectId}` | JWT |

### 11 · Chat Service (8 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/rooms` | JWT |
| GET | `/rooms/by-order/{orderId}` | JWT |
| GET | `/rooms/{roomId}` | JWT |
| GET / POST | `/rooms/{roomId}/messages` | JWT |
| POST | `/rooms/{roomId}/messages/upload` | JWT |
| POST | `/rooms/{roomId}/read` | JWT |
| GET | `/rooms/{roomId}/unread-count` | JWT |

### 12 · Search Service (5 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/search/merchants` | JWT |
| GET | `/search/menu-items` | JWT |
| GET | `/search/suggest` | JWT |
| GET | `/search/trending` | JWT |
| GET | `/search/popular` | JWT |

### 13 · Geo Service (14 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/maps/estimate` | JWT |
| GET | `/maps/polyline` | JWT |
| GET | `/maps/routing` | JWT |
| POST | `/maps/snapping` | JWT |
| GET | `/maps/traffic` | JWT |
| GET | `/maps/places/autocomplete` | JWT |
| GET | `/maps/places/details` | JWT |
| POST | `/maps/geocode` | JWT |
| POST | `/maps/reverse-geocode` | JWT |
| GET | `/distance` | JWT |
| POST | `/geofence/check` | JWT |
| GET / PUT | `/drivers/{driverId}/location` | JWT (driver/user) |
| GET | `/drivers/nearby` | JWT |

### 14 · Pricing Service (4 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/pricing/estimate/ride` | JWT |
| POST | `/pricing/estimate/food` | JWT |
| POST | `/pricing/estimate/delivery` | JWT |
| GET | `/pricing/surge` | JWT |

### 15 · Tracking Service (4 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/tracking/orders/{orderId}/position` | JWT |
| GET | `/tracking/orders/{orderId}/eta` | JWT |
| GET | `/tracking/orders/{orderId}/route` | JWT |
| GET | `/routes/{orderId}` | JWT |

### 16 · Notification Service (7 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/notifications` | JWT |
| GET | `/notifications/{notificationId}` | JWT |
| GET / POST | `/device-tokens` | JWT |
| DELETE | `/device-tokens/{tokenId}` | JWT |
| GET / PUT | `/preferences` | JWT |

### 17 · Matching Service — Driver Dispatcher (8 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/dispatcher/go-online` | JWT (driver) |
| POST | `/dispatcher/go-offline` | JWT (driver) |
| PUT | `/dispatcher/location` | JWT (driver) |
| POST | `/dispatcher/heartbeat` | JWT (driver) |
| POST | `/dispatcher/respond` | JWT (driver) |
| PUT | `/dispatcher/mode` | JWT (driver) |
| GET | `/dispatcher/status` | JWT (driver) |
| GET | `/dispatcher/earnings/today` | JWT (driver) |

### 18 · History Service (9 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/history/orders` | JWT |
| GET | `/history/orders/stats` | JWT |
| GET | `/history/orders/{orderId}` | JWT |
| GET | `/history/transactions` | JWT |
| GET | `/history/transactions/{transactionId}` | JWT |
| GET | `/feed` | JWT |
| GET | `/feed/{feedId}` | JWT |
| GET | `/driver/history/orders` | JWT (driver) |
| GET | `/driver/history/earnings` | JWT (driver) |

### 19 · Security Service (10 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/security/login-attempts` | JWT |
| GET | `/security/admin/login-attempts` | JWT (admin) |
| GET / POST | `/security/admin/fraud-flags` | JWT (admin) |
| GET | `/security/admin/fraud-flags/{flagId}` | JWT (admin) |
| POST | `/security/admin/fraud-flags/{flagId}/resolve` | JWT (admin) |
| GET / POST | `/security/admin/ip-blacklist` | JWT (admin) |
| DELETE | `/security/admin/ip-blacklist/{blockId}` | JWT (admin) |
| GET | `/security/admin/users/{userId}/fraud-summary` | JWT (admin) |

### 20 · Audit Log Service (7 endpoint)

| Method | Path | Auth |
|--------|------|------|
| GET | `/audit/admin/logs` | JWT (admin) |
| GET | `/audit/admin/logs/stats` | JWT (admin) |
| POST | `/audit/admin/logs/export` | JWT (admin) |
| GET | `/audit/admin/logs/export/{exportId}` | JWT (admin) |
| GET | `/audit/admin/logs/by-actor/{actorId}` | JWT (admin) |
| GET | `/audit/admin/logs/by-resource/{resourceType}/{resourceId}` | JWT (admin) |
| GET | `/audit/admin/logs/{logId}` | JWT (admin) |

### 21 · Webhooks — Provider Callbacks (2 endpoint)

| Method | Path | Auth |
|--------|------|------|
| POST | `/webhooks/email/delivery` | Public (signature) |
| POST | `/webhooks/sms/delivery` | Public (signature) |

### 22 · Internal — Service-to-Service (9 endpoint)

> Endpoint ini tidak melewati gateway auth standar. Dipanggil antar-service secara internal.

| Method | Path | Service Target |
|--------|------|----------------|
| POST | `/users/internal/users/lookup-by-phone` | User |
| GET | `/food/internal/orders/{orderId}` | Food Order |
| POST | `/food/internal/orders/{orderId}/assign-driver` | Food Order |
| GET | `/food/internal/users/{userId}/active-order` | Food Order |
| GET | `/merchants/internal/merchants/{merchantId}` | Merchant |
| GET | `/merchants/internal/merchants/{merchantId}/is-open` | Merchant |
| POST | `/merchants/internal/menu-items/batch` | Merchant |
| POST | `/dispatcher/internal/dispatcher/dispatch` | Matching |
| POST | `localhost:8014/internal/push/send` | Push |

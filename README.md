# Clay Platform

## Cara jalankan pipeline
1. Jalankan docker desktop
2. Jalankan minikube
    ```cmd
    minikube start --driver=docker
    ```
3. Jalankan jenkins
4. Buat kredential dockerhub di jenkins dengan nama `dockerhub-cred` (wajib sama biar automation jalan)
5. Buat pipeline di jenkins
   - Pilih pipeline script from SCM
   - Masukkan repo link repo "https://github.com/zicofarry/clay-tubes"
   - Pilih branch develop (agar tiap servicenya melakukan build, test, push, dan deploy ke kubernetes)
6. Build Now pada pipeline
7. Check console output/ pipeline overview untuk melihat hasil build


## Structure

```
clay-tubes/
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
├── Jenkinsfile              # Single CI/CD pipeline
└── README.md
```

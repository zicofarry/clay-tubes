# clay-payment-service

buat jalanain vendor

```
go mod vendor
```

buat jalanin unit test

```
go test -tags=unit -v ./...
```

buat jalanin build image

```
docker build -t registry.clay.id/clay-payment-service:latest -f Dockerfile .
```


buat jalanin docker

```
docker compose up -d
```


buat jalanin functional test

```
go test -tags=functional -v ./...
```

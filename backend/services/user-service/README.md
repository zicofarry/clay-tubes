# clay-user-service

buat jalanin vendor

```
go mod vendor
```

buat jalanin unit test

```
go test -tags=unit -v ./...
```

buat jalanin build image

```
docker build -t registry.clay.id/clay-user-service:latest -f Dockerfile .
```


buat jalanin docker

```
docker compose up -d
```


buat jalanin functional test

```
go test -count=1 -tags=functional -v ./...
```

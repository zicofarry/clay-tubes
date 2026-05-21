# clay-search-service

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
docker build -t registry.clay.id/clay-search-service:latest -f Dockerfile .
```


buat jalanin docker

```
docker compose up -d
```


buat jalanin functional test

```
go test -tags=functional -v ./...
```





# Clay Search Service

This is the search service for the Clay application, responsible for indexing and searching merchants and menu items using Elasticsearch.

## Prerequisites

- Go 1.25.0 or later
- Docker & Docker Compose (for running dependencies like Elasticsearch & Redis)
- `mockgen` (for generating mocks, installed via `go install go.uber.org/mock/mockgen@latest`)

## Running the Tests

We have two main types of tests: **Unit Tests** and **Functional/Integration Tests**.

### 1. Running Unit Tests

Unit tests are used to test individual functions, methods, or layers (Handler, Service, Repository) in isolation. We use `gomock` to simulate dependencies (e.g., mocking the database inside the Service tests).

To run all unit tests in the command prompt:

```bash
# Navigate to the service directory
cd D:\Github\clay\clay-search-service

# Run all tests recursively with verbose output
go test ./internal/... -v
```

### 2. Running Functional Tests & Elasticsearch Integration

Functional tests evaluate how multiple components interact. If you want to do full end-to-end (E2E) testing that hits a real Elasticsearch instance, follow these steps:

#### Step 1: Start the Elasticsearch Container
We have a `docker-compose.yml` file ready to spin up Elasticsearch locally.

```bash
# Navigate to the service directory
cd D:\Github\clay\clay-search-service

# Start the containers in the background (-d)
docker-compose up -d
```
*Wait a few seconds for Elasticsearch to become fully active.*

You can verify that Elasticsearch is running by opening a browser or running `curl` to `http://localhost:9200`.

#### Step 2: Run the Functional Tests
Once Elasticsearch is running, you can execute the functional test suite. Currently, the `search_integration_test.go` uses mock dependencies, but you can build upon it to test real connections.

```bash
# Run the functional test suite
go test ./test/functional/... -v
```

#### Step 3: Tear down the Containers
After you are done testing, don't forget to stop and remove the containers to free up resources:

```bash
docker-compose down
```

## Generating Mocks

If you modify any interfaces inside the `internal/` directory, you need to regenerate the mocks so the unit tests don't break.

```bash
go mod vendor
mockgen -source=internal/service/search_service.go -destination=mocks/mock_search_service.go -package=mocks
mockgen -source=internal/repository/search_repository.go -destination=mocks/repomock/mock_search_repository.go -package=repomock
```

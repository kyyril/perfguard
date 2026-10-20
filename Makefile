.PHONY: all build fmt fmt-check lint vet test test-race test-integration test-e2e cover vuln eval check tidy

BIN := bin/perfguard

all: build

build:
	go build -o $(BIN) ./cmd/perfguard

fmt:
	gofmt -s -w .

fmt-check:
	@gofmt -l .

lint:
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test -shuffle=on -count=1 ./...

test-race:
	go test -race -shuffle=on -count=1 ./...

test-integration:
	go test -tags=integration -race -count=1 ./...

test-e2e:
	go test -tags=e2e -count=1 ./test/e2e/...

cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

vuln:
	govulncheck ./...

eval:
	@echo Evaluation suite not run in basic check

check: vet lint test-race vuln

tidy:
	go mod tidy

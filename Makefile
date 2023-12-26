-include .env

run:
	@go run cmd/main.go

test:
	@go test -v ./...

coverage:
	@go test -cover ./...

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/app cmd/main.go

compose_test:
	sudo docker compose up togglelabs_test_db

.PHONY: run test coverage build
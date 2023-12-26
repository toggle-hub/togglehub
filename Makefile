-include .env

run:
	go run cmd/main.go

test:
	go test -v ./...

coverage:
	go test -cover ./...

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/app cmd/main.go


.PHONY: migratecreate migrateup migratedown run test coverage build
-include .env

migratecreate:
	migrate create -ext sql -dir migrations -seq $(name)

migrateup:
	migrate -path migrations/ -database ${DB_URL} -verbose up

migratedown:
	migrate -path migrations/ -database ${DB_URL} -verbose down 1

run:
	go run cmd/main.go

test:
	go test -v ./...

coverage:
	go test -cover ./...

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/app cmd/main.go


.PHONY: migratecreate migrateup migratedown run test coverage build
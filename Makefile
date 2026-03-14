.PHONY: run build test

run:
	go run main.go

build:
	go build -o bin/battleship-backend main.go

test:
	go test ./...

tidy:
	go mod tidy

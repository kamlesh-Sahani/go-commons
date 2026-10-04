.PHONY: run-server run-example build-server test docker-up docker-down clean

run-server:
	go run ./cmd/server

run-example:
	go run ./examples

build-server:
	go build -o bin/server ./cmd/server

test:
	go test -v ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/

.PHONY: run-example test clean

run-example:
	go run ./examples

test:
	go test -v ./...

clean:
	go clean -cache -testcache

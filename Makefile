.PHONY: build test race lint cover clean run

BIN := bin/flowcmd

build:
	go build -o $(BIN) .

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

run: build
	./$(BIN) run examples/hello.yml

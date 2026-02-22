.PHONY: build run clean test

BINARY := governor
MAIN := ./cmd/governor

build:
	go build -o $(BINARY) $(MAIN)

run:
	go run $(MAIN)

clean:
	rm -f $(BINARY)

test:
	go test ./...

deps:
	go mod download
	go mod tidy

lint:
	go fmt ./...
	go vet ./...

install: build
	mv $(BINARY) /usr/local/bin/

.PHONY: docker
docker:
	docker build -t vibepilot/governor:latest .

.PHONY: cross
cross:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 $(MAIN)
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 $(MAIN)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 $(MAIN)

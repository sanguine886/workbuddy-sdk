.PHONY: all build vet test fmt tidy clean

all: vet test

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

clean:
	go clean ./...

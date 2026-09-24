.PHONY all: build test vet

build:
	go build ./cmd/marvin
test:
	go test -v ./...
vet:
	go vet ./...

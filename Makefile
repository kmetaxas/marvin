.PHONY all: build test vet

build:
	CGO_ENABLED=0 go build ./cmd/marvin
test:
	go test -v ./...
vet:
	go vet ./...

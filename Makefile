.PHONY: build test cover covercheck

BINARY := githerd
COVER_MIN := 80

build:
	go build -o $(BINARY) ./cmd/githerd

test:
	go test ./...

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

covercheck: cover
	@go run ./scripts/covercheck.go -min=$(COVER_MIN) -profile=coverage.out

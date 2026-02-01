.PHONY: docs format run build vet

SWAG_BIN=~/go/bin/swag
MAIN_FILE=cmd/api/main.go
DOCS_OUTPUT_DIR=./api/docs

dev:
	air

run:
	go run cmd/api/main.go

# Testing
test:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Quality
lint:
	golangci-lint run ./...

vet:
	go vet ./...

format:
	go fmt ./...
	goimports -w .

# Documentation
docs:
	$(SWAG_BIN) init -g $(MAIN_FILE) --parseDependency --parseInternal --parseVendor -o $(DOCS_OUTPUT_DIR)

# Build
build:
	go build -o ./bin/medusa ./cmd/api

clean:
	rm -rf ./bin ./tmp coverage.out coverage.html
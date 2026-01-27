.PHONY: install generate generate-go generate-ts run-server dev drop-schema

# Install dependencies
install:
	cd app && bun install
	go mod download

# Vendor dependencies
vendor:
	go mod tidy
	go mod vendor

# Generate code from proto files
generate: generate-go generate-ts

# Generate Go code from proto
generate-go:
	rm -rf server/gen
	cd proto && npx buf generate --template buf.gen.go.yaml

# Generate TypeScript code from proto
generate-ts:
	rm -rf app/gen
	cd proto && npx buf generate --template buf.gen.ts.yaml

# Run the Go server
run-server:
	go run cmd/server/main.go

# Development: run server
dev: run-server

# Drop database schema
drop-schema:
	go run cmd/server/main.go -drop

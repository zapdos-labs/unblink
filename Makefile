.PHONY: install proto proto-go proto-ts dev drop-schema typecheck docker-build docker-run

# Install dependencies
install:
	cd app && bun install
	go mod download

# Vendor dependencies
vendor:
	go mod tidy
	go mod vendor

# Generate code from proto files
proto:
	rm -rf app/gen
	cd proto && buf generate --template buf.gen.ts.yaml
	rm -rf server/gen
	cd proto && buf generate --template buf.gen.go.yaml

# Drop database schema
drop:
	go run cmd/cli/main.go drop

# Typecheck (ts and go)
typecheck:
	cd app && bunx tsc --noEmit
	go vet ./...

# Development (tmux session with server + node + app)
dev:
	./tmux.dev.sh

delete-app-dir:
	go run cmd/cli/main.go delete-app-dir

# Docker commands for deployment
docker-build:
	docker build -t unblink-v2:local .

docker-run: docker-build
	docker run -p 8080:8080 \
		-e DATABASE_URL="$(DATABASE_URL)" \
		-e JWT_SECRET="$(JWT_SECRET)" \
		-e DASHBOARD_URL="http://localhost:8080" \
		-e CHAT_OPENAI_MODEL="$(CHAT_OPENAI_MODEL)" \
		-e CHAT_OPENAI_BASE_URL="$(CHAT_OPENAI_BASE_URL)" \
		-e CHAT_OPENAI_API_KEY="$(CHAT_OPENAI_API_KEY)" \
		-e VLM_OPENAI_MODEL="$(VLM_OPENAI_MODEL)" \
		-e VLM_OPENAI_BASE_URL="$(VLM_OPENAI_BASE_URL)" \
		-e VLM_OPENAI_API_KEY="$(VLM_OPENAI_API_KEY)" \
		unblink-v2:local

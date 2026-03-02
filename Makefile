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
	bash -lc 'set -a; source ./.env; set +a; go run cmd/cli/main.go drop'

# Typecheck (ts and go)
typecheck:
	cd app && bunx tsc --noEmit
	go vet ./...

# Development (tmux session with server + node + app)
dev:
	./tmux.dev.sh

delete-app-dir:
	bash -lc 'set -a; source ./.env; set +a; go run cmd/cli/main.go delete-app-dir'

# Docker commands for deployment
docker-build:
	docker build -t unblink:local .

docker-run: docker-build
	env -i PATH="$$PATH" HOME="$$HOME" bash -lc 'set -a; source ./.env; set +a; \
		port="$${VITE_SERVER_API_PORT:-8080}"; \
		docker run --env-file <(env) \
			-p "$$port:$$port" \
			-e "DASHBOARD_URL=http://localhost:$$port" \
			unblink:local'

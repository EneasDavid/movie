.PHONY: install build build-web build-server run dev fmt vet test clean

# Full production-equivalent build: React/SCSS first (Vite writes into
# internal/web/static), then the Go binary embeds whatever is there.
# Order matters — running `go build` without this first just re-embeds
# the committed placeholder page.
build: build-web build-server

install:
	npm --prefix web ci

build-web:
	npm --prefix web run build

build-server:
	go build -o bin/server ./cmd/server

run: build
	./bin/server

# Two dev servers, hot-reloading independently: Vite on :5173 (proxies
# /api/* to the Go server per web/vite.config.js), Go on :8080.
dev:
	@echo "Run in two terminals: 'make dev-web' and 'make dev-server'"

dev-web:
	npm --prefix web run dev

dev-server:
	go run ./cmd/server

fmt:
	gofmt -l .
	npm --prefix web run lint

vet:
	go vet ./cmd/... ./internal/...

test:
	go test ./cmd/... ./internal/...

clean:
	rm -rf bin web/node_modules internal/web/static/assets

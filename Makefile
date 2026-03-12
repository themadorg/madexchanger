.PHONY: build test lint clean fmt vet admin-web stage-admin-web all push log

# Load environment variables (contains server IPs, gitignored)
-include .env
export

VERSION ?= dev
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
BINARY := madexchanger
ADMIN_WEB_SRC := admin-web
ADMIN_WEB_BUILD := admin-web/build
ADMIN_WEB_DEST := internal/adminweb/build

## build — Compile the madexchanger binary (with CGO for SQLite).
##         The admin-web SPA must be staged first (use 'make all').
build: fmt vet
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY) ./cmd/madexchanger

## test — Run all tests with race detection and verbose output.
test:
	CGO_ENABLED=1 go test -race -v ./...

## lint — Run golangci-lint (must be installed separately).
lint:
	golangci-lint run ./...

## fmt — Format all Go source files.
fmt:
	go fmt ./...

## vet — Run go vet on all packages.
vet:
	go vet ./...

## admin-web — Build the SvelteKit admin dashboard SPA.
admin-web:
	@if [ -f "$(ADMIN_WEB_SRC)/package.json" ]; then \
		if command -v bun > /dev/null 2>&1; then \
			echo "-- Building admin-web (bun)..."; \
			cd $(ADMIN_WEB_SRC) && bun install && bun run build; \
		elif command -v npm > /dev/null 2>&1; then \
			echo "-- Building admin-web (npm)..."; \
			cd $(ADMIN_WEB_SRC) && npm install && npm run build; \
		else \
			echo "-- [!] No bun or npm found. Skipping admin-web build."; \
		fi \
	fi

## stage-admin-web — Copy the admin-web build output for go:embed.
##                    This is the "Stage-and-Embed" pattern from Madmail.
stage-admin-web: admin-web
	@if [ -d "$(ADMIN_WEB_BUILD)" ] && [ -f "$(ADMIN_WEB_BUILD)/index.html" ]; then \
		echo "-- Copying admin-web build to $(ADMIN_WEB_DEST)..."; \
		rm -rf "$(ADMIN_WEB_DEST)"; \
		cp -r "$(ADMIN_WEB_BUILD)" "$(ADMIN_WEB_DEST)"; \
	else \
		echo "-- [!] admin-web not built. Admin web UI will not be available."; \
		mkdir -p "$(ADMIN_WEB_DEST)"; \
		[ -f "$(ADMIN_WEB_DEST)/placeholder" ] || echo "placeholder" > "$(ADMIN_WEB_DEST)/placeholder"; \
	fi

## all — Build everything: admin-web → stage → Go binary.
all: stage-admin-web build

## clean — Remove build artifacts.
clean:
	rm -f $(BINARY)
	rm -rf $(ADMIN_WEB_DEST)
	mkdir -p $(ADMIN_WEB_DEST)
	echo "placeholder" > $(ADMIN_WEB_DEST)/placeholder

## push — Build and deploy to the exchanger server.
##         Uploads binary, restarts the service.
##         Server IP is set in .env (EXCHANGER1=x.x.x.x).
push: build
	@if [ -z "$(EXCHANGER1)" ]; then \
		echo "❌ EXCHANGER1 not set. Create .env with EXCHANGER1=<ip>"; \
		exit 1; \
	fi
	@echo "📦 Uploading madexchanger to $(EXCHANGER1)..."
	scp $(BINARY) root@$(EXCHANGER1):/usr/local/bin/madexchanger-new
	@echo "🔄 Restarting madexchanger on $(EXCHANGER1)..."
	ssh root@$(EXCHANGER1) "mv /usr/local/bin/madexchanger-new /usr/local/bin/madexchanger && systemctl restart madexchanger"
	@echo "✅ Deployed and restarted on $(EXCHANGER1)"

## log — Tail the madexchanger logs on the remote server.
log:
	ssh root@$(EXCHANGER1) "journalctl -u madexchanger -f"

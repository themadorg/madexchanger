.PHONY: build test lint clean fmt vet admin-web stage-admin-web all

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

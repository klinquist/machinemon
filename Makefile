VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w \
	-X github.com/machinemon/machinemon/internal/version.Version=$(VERSION) \
	-X github.com/machinemon/machinemon/internal/version.Commit=$(COMMIT) \
	-X github.com/machinemon/machinemon/internal/version.BuildTime=$(BUILD_TIME)"

.PHONY: all clean web build-client build-server dev-client dev-server test lint release prepare-binaries

all: web build-client build-server

# Build React SPA
web:
	cd web && npm ci && npm run build
	rm -rf cmd/machinemon-server/web_dist
	cp -r web/dist cmd/machinemon-server/web_dist

# Build client for current platform
dev-client:
	go build $(LDFLAGS) -o dist/machinemon-client ./cmd/machinemon-client

# Build server for current platform
dev-server: web
	go build $(LDFLAGS) -o dist/machinemon-server ./cmd/machinemon-server

# Cross-compile client for all targets
build-client:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build $(LDFLAGS) -o dist/machinemon-client-linux-armv6 ./cmd/machinemon-client
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o dist/machinemon-client-linux-armv7 ./cmd/machinemon-client
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/machinemon-client-linux-arm64 ./cmd/machinemon-client
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/machinemon-client-linux-amd64 ./cmd/machinemon-client
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/machinemon-client-darwin-amd64 ./cmd/machinemon-client
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/machinemon-client-darwin-arm64 ./cmd/machinemon-client

# Cross-compile server for all targets
build-server: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/machinemon-server-linux-amd64 ./cmd/machinemon-server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/machinemon-server-linux-arm64 ./cmd/machinemon-server
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/machinemon-server-darwin-amd64 ./cmd/machinemon-server
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/machinemon-server-darwin-arm64 ./cmd/machinemon-server

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

# Create release archives
release: all
	cd dist && for f in machinemon-client-* machinemon-server-*; do \
		case "$$f" in *.tar.gz) continue ;; esac; \
		COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -czf "$$f.tar.gz" "$$f"; \
	done
	cd dist && shasum -a 256 *.tar.gz > checksums.txt

# Package client binaries for server-hosted distribution
# Copy the resulting binaries/ directory to your server's binaries_dir
prepare-binaries: build-client
	mkdir -p binaries
	cd dist && for f in machinemon-client-*; do \
		case "$$f" in *.tar.gz) continue ;; esac; \
		COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -czf "../binaries/$$f.tar.gz" "$$f"; \
	done
	@echo ""
	@echo "Client binaries ready in binaries/"
	@echo "Copy to your server's binaries_dir (see server.toml)."
	@echo ""
	@ls -lh binaries/

clean:
	rm -rf dist/ binaries/ web/dist/ cmd/machinemon-server/web_dist/

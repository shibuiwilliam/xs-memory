BINARY    := xsmem
CMD       := ./cmd/xsmem
BINDIR    := bin
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

GO       := CGO_ENABLED=0 go
GOTEST   := CGO_ENABLED=0 go test
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: build build-all run test test-short bench lint fmt tidy webui clean install-hooks

## build: compile for the current platform (pure Go, no CGO)
build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINARY) $(CMD)

## build-all: cross-compile for all supported OS/arch
build-all:
	@for plat in $(PLATFORMS); do \
		os=$${plat%/*}; arch=$${plat#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "Building $$os/$$arch ..."; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' \
			-o $(BINDIR)/$(BINARY)-$$os-$$arch$$ext $(CMD) || exit 1; \
	done

## run: build and run with arguments (usage: make run -- <args>)
run: build
	./$(BINDIR)/$(BINARY) $(filter-out $@,$(MAKECMDGOALS))

## test: run all tests with race detector
test:
	$(GOTEST) ./... -race -count=1

## test-short: run only short/fast tests
test-short:
	$(GOTEST) ./... -race -short -count=1

## bench: run benchmarks
bench:
	$(GOTEST) ./... -bench=. -benchmem -run=^$$ -count=1

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format code with gofumpt and goimports
fmt:
	gofumpt -w .
	goimports -w .

## tidy: tidy go modules
tidy:
	go mod tidy

## webui: build with webui tag
webui:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -tags webui -o $(BINDIR)/$(BINARY) $(CMD)

## install-hooks: install git pre-commit hook
install-hooks:
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed."

## clean: remove build artifacts
clean:
	rm -rf $(BINDIR)

# catch-all for `make run` args
%:
	@:

VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
GO := go

.PHONY: build test vet fmt clean dist

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o hubtop ./cmd/hubtop

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

fmt:
	test -z "$$(gofmt -l .)"

clean:
	rm -rf hubtop dist/

dist: dist/hubtop-linux-amd64 dist/hubtop-linux-arm64 \
      dist/hubtop-darwin-amd64 dist/hubtop-darwin-arm64 \
      dist/hubtop-windows-amd64.exe dist/hubtop-windows-arm64.exe

dist/hubtop-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

dist/hubtop-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

dist/hubtop-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

dist/hubtop-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

dist/hubtop-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

dist/hubtop-windows-arm64.exe:
	GOOS=windows GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/hubtop

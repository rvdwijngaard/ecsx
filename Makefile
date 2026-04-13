.PHONY: build install clean

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev-$$(git rev-parse --short HEAD)")
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o ecsx ./cmd/ecsx

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/ecsx

clean:
	rm -f ecsx

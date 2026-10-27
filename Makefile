VERSION ?= $(shell cat VERSION)
GO ?= go
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test race vet fmt clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bambulab-exporter .

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w *.go

clean:
	rm -f bambulab-exporter

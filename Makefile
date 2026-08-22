# Development helpers for ThreadfinX.
#
# `go test` runs `go vet` as part of its build step, so a vet failure makes the
# whole package unbuildable for tests. Keep `make vet` green or `make race`
# cannot run at all.

GO      ?= go
PKGS    ?= ./...
ARM_OUT ?= dist/Threadfin_linux_arm64

.PHONY: vet test race build build-arm64 check

vet:
	$(GO) vet $(PKGS)

test:
	$(GO) test $(PKGS)

# The race detector is the tool of record for the buffer/streaming code.
race:
	$(GO) test -race -count=1 $(PKGS)

build:
	$(GO) build $(PKGS)

# Production target: Raspberry Pi 4 / ARM64.
build-arm64:
	mkdir -p $(dir $(ARM_OUT))
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-s -w" -o $(ARM_OUT) .

check: vet race build-arm64

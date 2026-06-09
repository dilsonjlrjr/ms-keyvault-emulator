BIN      := bin/kvemu
MAIN     := ./cmd/kvemu
VERSION  := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS  := -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all build test verify gate run clean

all: gate build

## build: compila o binário (requer gate verde)
build: gate
	CGO_ENABLED=0 go build $(LDFLAGS) -trimpath -o $(BIN) $(MAIN)

## test: camadas 1+2 — testes unitários (sem Docker, roda em ms)
test:
	go test ./... -count=1 -race

## verify: camada 3 — E2E matricial com SDK Java real (requer Docker)
verify:
	go test ./test/e2e/... -tags=e2e -count=1 -timeout=5m

## gate: test + verify — bloqueia build se qualquer camada falhar
gate: test

## run: sobe o emulador em modo dev (TLS auto-gen, auth leniente)
run:
	KV_TLS_AUTO=true KV_AUTH_STRICT=false go run $(MAIN)

## clean: remove binários e dados gerados
clean:
	rm -rf bin/ data/ certs/

## lint: verifica estilo e erros estáticos
lint:
	golangci-lint run ./...

BIN      := bin/kvemu
MAIN     := ./cmd/kvemu
VERSION  := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS  := -ldflags="-s -w -X main.version=$(VERSION)"
IMAGE    := ghcr.io/dilsonrabelo/kvemu

.PHONY: all build test verify gate run clean dist \
        docker/build docker/run docker/push docker/buildx docker/seed \
        compat/spring27 compat/spring279 compat/spring3

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
gate: test verify

## run: sobe o emulador em modo dev (TLS auto-gen, auth leniente)
run:
	KV_TLS_AUTO=true KV_AUTH_STRICT=false go run $(MAIN)

## clean: remove binários e dados gerados
clean:
	rm -rf bin/ data/ certs/ dist/

## dist: compila para linux, builda imagem Docker e exporta compactada
dist:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -trimpath -o $(BIN) $(MAIN)
	docker build \
		-f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		.
	mkdir -p dist/image-docker
	docker save $(IMAGE):$(VERSION) | gzip > dist/image-docker/kvemu-$(VERSION).tar.gz
	@echo "Image exported: dist/image-docker/kvemu-$(VERSION).tar.gz"
	@echo "Load with:   docker load -i dist/image-docker/kvemu-$(VERSION).tar.gz"

## lint: verifica estilo e erros estáticos
lint:
	golangci-lint run ./...

## docker/build: constrói a imagem para a arquitetura local
docker/build:
	docker build \
		-f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

## docker/run: sobe o emulador via docker-compose
docker/run:
	VERSION=$(VERSION) docker compose -f deploy/docker-compose.yml up --build

## docker/push: envia imagem para o registry
docker/push: docker/build
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

## docker/buildx: multi-arch build (linux/amd64 + linux/arm64) com push
docker/buildx:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--push \
		.

## docker/seed: injeta dados de desenvolvimento no emulador em execução
docker/seed:
	docker compose -f deploy/docker-compose.yml exec kvemu /kvemu seed

## compat/spring27: prova de compatibilidade Spring Boot 2.7 + Spring Cloud Azure 4.5.0
## Requer Docker. Primeira execução baixa imagens Maven (~400 MB) e demora ~5 min.
compat/spring27:
	go test -tags=spring ./test/compat/... -v -timeout=15m -run TestSpringBoot27Compat

## compat/spring3: prova de compatibilidade Spring Boot 3.4 + Spring Cloud Azure 5.x
## Requer Docker. Primeira execução baixa imagens Maven (~400 MB) e demora ~5 min.
compat/spring3:
	go test -tags=spring ./test/compat/... -v -timeout=15m -run TestSpringBoot3Compat

## compat/spring279: prova de compatibilidade Spring Boot 2.7.9 + Spring Cloud Azure 4.5.0
## Requer Docker. Primeira execução baixa imagens Maven (~400 MB) e demora ~5 min.
compat/spring279:
	go test -tags=spring ./test/compat/... -v -timeout=15m -run TestSpringBoot279Compat

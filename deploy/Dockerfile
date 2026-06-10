# ─── stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /build

# deps first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -trimpath \
      -o /kvemu \
      ./cmd/kvemu

# cria dirs de dados com ownership do nonroot (UID 65532) para que named volumes
# sejam inicializados com permissões corretas no primeiro uso do container
RUN mkdir -p /data /certs && chown 65532:65532 /data /certs

# ─── stage 2: distroless (sem shell, sem libc, ~7MB) ────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /kvemu /kvemu

# dirs owned por nonroot → Docker inicializa named volumes com ownership correto
COPY --from=builder --chown=65532:65532 /data  /data
COPY --from=builder --chown=65532:65532 /certs /certs

VOLUME ["/data", "/certs"]

EXPOSE 13000

# kvemu healthcheck faz loopback HTTPS com InsecureSkipVerify
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/kvemu", "healthcheck", "--addr", "https://localhost:13000"]

ENTRYPOINT ["/kvemu"]

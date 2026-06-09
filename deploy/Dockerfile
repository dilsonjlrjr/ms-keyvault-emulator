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

# ─── stage 2: distroless (sem shell, sem libc, ~7MB) ────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /kvemu     /kvemu
COPY --from=builder /build/migrations /migrations

EXPOSE 13000

# kvemu healthcheck faz loopback HTTPS com InsecureSkipVerify
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/kvemu", "healthcheck", "--addr", "https://localhost:13000"]

ENTRYPOINT ["/kvemu"]

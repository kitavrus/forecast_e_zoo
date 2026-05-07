# syntax=docker/dockerfile:1.6

# ---------- builder ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=docker" \
    -o /out/source-adapter ./cmd/source-adapter

# ---------- runner ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/source-adapter /app/source-adapter

USER nonroot:nonroot

EXPOSE 8080

VOLUME ["/var/exports"]

ENTRYPOINT ["/app/source-adapter"]

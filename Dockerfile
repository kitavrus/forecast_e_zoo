# syntax=docker/dockerfile:1.6
# Unified multi-stage Dockerfile для всех 7 микросервисов e_zoo.
# Использование:
#   docker build --build-arg BINARY_NAME=source-adapter -t ezoo/source-adapter .
#   docker build --build-arg BINARY_NAME=etl            -t ezoo/etl .
#   docker build --build-arg BINARY_NAME=data-marts     -t ezoo/data-marts .
#   docker build --build-arg BINARY_NAME=kpi            -t ezoo/kpi .
#   docker build --build-arg BINARY_NAME=forecast       -t ezoo/forecast .
#   docker build --build-arg BINARY_NAME=order-builder  -t ezoo/order-builder .
#   docker build --build-arg BINARY_NAME=channel-router -t ezoo/channel-router .

# ---------- builder ----------
FROM golang:1.26-alpine AS builder
ARG BINARY_NAME
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS="-trimpath"
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN test -n "$BINARY_NAME" || (echo "BINARY_NAME build-arg is required" && exit 1)
RUN go build \
    -ldflags="-s -w -X main.version=docker" \
    -o /out/service ./cmd/${BINARY_NAME}

# ---------- runner ----------
FROM alpine:3.20
ARG BINARY_NAME
RUN apk add --no-cache ca-certificates tzdata curl && update-ca-certificates
RUN adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /out/service /app/service
COPY configs /app/configs
COPY testdata /app/testdata

ENV SERVICE_NAME=${BINARY_NAME} \
    TZ=Europe/Kyiv

USER appuser
ENTRYPOINT ["/app/service"]

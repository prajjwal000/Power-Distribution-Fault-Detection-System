# Build stage for Go binaries
FROM golang:1.26-alpine AS go-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /generator ./cmd/generator
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /simulator ./cmd/simulator

# Build stage for frontend (only needed for nginx)
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json frontend/pnpm-lock.yaml* ./
RUN corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile

COPY frontend/ ./
ENV CI=true
RUN pnpm run build

# API final stage (Go binary only, no frontend)
FROM alpine:3.19 AS api-final

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy Go binary
COPY --from=go-builder /api .

# Copy database schema
COPY internal/db/schema.sql internal/db/schema.sql
RUN mkdir -p data

# Simulator final stage (Go only, no frontend)
FROM alpine:3.19 AS simulator-final

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy Go binary only
COPY --from=go-builder /simulator .

# Seed final stage (Go only, no frontend)
FROM alpine:3.19 AS seed-final

RUN apk add --no-cache ca-certificates wget postgresql-client

WORKDIR /app

# Copy Go binaries
COPY --from=go-builder /generator .
COPY --from=go-builder /api .
COPY --from=go-builder /simulator .

# Copy database schema
COPY internal/db/schema.sql internal/db/schema.sql
RUN mkdir -p data
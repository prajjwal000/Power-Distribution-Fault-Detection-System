# Build stage for Go binaries
FROM golang:1.26-alpine AS go-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /generator ./cmd/generator
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /simulator ./cmd/simulator

# Build stage for frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json frontend/pnpm-lock.yaml* ./
RUN corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy Go binaries
COPY --from=go-builder /generator .
COPY --from=go-builder /api .
COPY --from=go-builder /simulator .

# Copy frontend build output
COPY --from=frontend-builder /frontend/dist ./static

# Copy database schema
COPY internal/db/schema.sql internal/db/schema.sql
RUN mkdir -p data

# The API binary will serve static files from ./static
# and handle API routes
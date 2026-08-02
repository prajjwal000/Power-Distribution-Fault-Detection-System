FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /generator ./cmd/generator
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /simulator ./cmd/simulator

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /generator .
COPY --from=builder /api .
COPY --from=builder /simulator .
COPY internal/db/schema.sql internal/db/schema.sql
RUN mkdir -p data

FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /solidgo ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /solidgo /usr/local/bin/solidgo

ENV SOLID_PORT=3000
ENV SOLID_STORAGE_PATH=/data

EXPOSE 3000

VOLUME ["/data"]

CMD ["solidgo", "-port", "3000", "-storage", "/data"]

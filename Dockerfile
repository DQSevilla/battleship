# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o battleship .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/battleship .
COPY --from=builder /app/web ./web

# Create data directory for SQLite
RUN mkdir -p /data

ENV PORT=8080
ENV DB_PATH=/data/battleship.db

EXPOSE 8080

CMD ["./battleship"]

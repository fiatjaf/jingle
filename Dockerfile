# Build stage
FROM golang:latest as builder

WORKDIR /build

RUN go install github.com/fiatjaf/jingle@latest

# Runtime stage
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /go/bin/jingle /app/jingle

RUN chmod +x /app/jingle

RUN mkdir ./data ./stuff

CMD ["/app/jingle"]

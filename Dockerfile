FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/payment-gateway

FROM alpine:3.21

RUN apk update && apk upgrade
RUN rm -rf /var/cache/apk/* /tmp/*

RUN adduser -D appuser
USER appuser

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]

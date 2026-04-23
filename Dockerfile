FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o personal-know ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -h /app appuser
WORKDIR /app
COPY --from=builder /app/personal-know .
RUN chown appuser:appuser ./personal-know
USER appuser
EXPOSE 8081
CMD ["./personal-know"]

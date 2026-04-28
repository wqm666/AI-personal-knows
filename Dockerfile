FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o personal-know ./cmd/server/

FROM alpine:3.20
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/personal-know .
EXPOSE 8081
CMD ["./personal-know"]

FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /tdx-api ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ENV TZ=Asia/Shanghai
ENV PORT=8080
COPY --from=builder /tdx-api /usr/local/bin/tdx-api
EXPOSE 8080
ENTRYPOINT ["tdx-api"]

FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /kaoshi ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /kaoshi /usr/local/bin/kaoshi
ENV TZ=Asia/Shanghai
EXPOSE 8080
ENTRYPOINT ["kaoshi"]

FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/weedpanel ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/weedpanel /app/weedpanel
USER app
EXPOSE 8081
ENTRYPOINT ["/app/weedpanel"]

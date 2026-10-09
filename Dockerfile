FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o dns-test .

FROM scratch
COPY --from=builder /app/dns-test /dns-test
EXPOSE 8080
ENTRYPOINT ["/dns-test"]

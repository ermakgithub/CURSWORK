FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o logistics-app ./main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/logistics-app /app/logistics-app
COPY --from=builder /app/static /app/static
RUN mkdir -p /app/data /app/logs
EXPOSE 8080
CMD ["/app/logistics-app"]

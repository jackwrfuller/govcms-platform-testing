FROM golang:1.25-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /platform-testing ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /platform-testing /app/platform-testing
COPY web/ /app/web/
COPY sites.yml /app/sites.yml

ENV PORT=3000
EXPOSE 3000

ENTRYPOINT ["/app/platform-testing"]

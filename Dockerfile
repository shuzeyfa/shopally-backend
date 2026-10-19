FROM golang:1.24.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S shopally && adduser -S -G shopally shopally
WORKDIR /app
COPY --from=build --chown=shopally:shopally /out/api /app/api
COPY --from=build --chown=shopally:shopally /out/worker /app/worker
USER shopally
EXPOSE 10000
CMD ["/app/api"]

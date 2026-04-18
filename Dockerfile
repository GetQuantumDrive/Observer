FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /observer ./cmd/action

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git
COPY --from=build /observer /observer
ENTRYPOINT ["/observer"]

FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /observer ./cmd/action

FROM alpine:3.20 AS rules
ARG OBSERVER_RULES_REF=main
RUN apk add --no-cache git \
 && git clone --depth 1 --branch ${OBSERVER_RULES_REF} \
      https://github.com/GetQuantumDrive/Observer-rules /tmp/rules-src \
 && mkdir -p /opt/observer/rules \
 && find /tmp/rules-src \( -name '*.yaml' -o -name '*.yml' \) -exec cp {} /opt/observer/rules/ \;

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git
COPY --from=build /observer /observer
COPY --from=rules /opt/observer/rules /opt/observer/rules
ENV OBSERVER_BUNDLED_RULES_DIR=/opt/observer/rules
ENTRYPOINT ["/observer"]

FROM golang:alpine as builder
ARG LDFLAGS=""

RUN apk --update --no-cache add git build-base gcc

COPY . /build
WORKDIR /build

RUN go build -ldflags "${LDFLAGS}" -o plane-module cmd/main.go

FROM alpine:latest
ARG src_dir

RUN apk update --no-cache && \
    apk add tzdata curl jq && \
    adduser -S -D -H -h / aruba
USER plane
WORKDIR /app
COPY --from=builder /build/plane-module /app

ENTRYPOINT ["/app/plane-module"]
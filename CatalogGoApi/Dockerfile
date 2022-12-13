# syntax=docker/dockerfile:1

## Build
FROM golang:1.17-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY src/*.go ./

RUN go build -o /catalog-api

## Deploy
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /catalog-api /catalog-api

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/catalog-api"]
FROM docker.io/library/golang:1.22-alpine AS build
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY ./cmd ./cmd
COPY *.go ./

RUN go build -o /magpie ./cmd/magpie

FROM docker.io/library/alpine:latest
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

COPY --from=build /magpie /magpie

CMD ["/magpie"]

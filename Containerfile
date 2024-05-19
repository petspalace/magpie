FROM docker.io/library/golang:1.22-alpine AS build
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o /magpie

FROM docker.io/library/alpine:latest
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

COPY --from=build /magpie /magpie

CMD ["/magpie"]

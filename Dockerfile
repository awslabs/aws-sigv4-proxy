FROM golang:1.11-alpine3.8 as build

RUN apk add -U --no-cache ca-certificates git bash

COPY ./ /go/src/github.com/awslabs/aws-sigv4-proxy
WORKDIR /go/src/github.com/awslabs/aws-sigv4-proxy

RUN go build -o app github.com/awslabs/aws-sigv4-proxy && \
    mv ./app /go/bin

FROM alpine:3.8

WORKDIR /opt/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /opt/

ENTRYPOINT [ "/opt/app" ]

FROM golang:alpine3.12 as build

RUN apk add -U --no-cache ca-certificates git bash

RUN mkdir /aws-sigv4-proxy
WORKDIR /aws-sigv4-proxy
COPY go.mod .
COPY go.sum .

RUN go env -w GOPROXY=direct
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/aws-sigv4-proxy

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/aws-sigv4-proxy /go/bin/aws-sigv4-proxy

ENTRYPOINT [ "/go/bin/aws-sigv4-proxy" ]

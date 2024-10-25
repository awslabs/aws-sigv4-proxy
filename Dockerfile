FROM golang:1.22.4-alpine AS build

RUN apk --update add \
      ca-certificates \
      git

RUN mkdir /aws-sigv4-proxy
WORKDIR /aws-sigv4-proxy
COPY  . .

RUN go env -w GOPROXY=direct

RUN go get golang.org/x/time/rate

RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/aws-sigv4-proxy

FROM alpine:3
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /aws-sigv4-proxy/aws-sigv4-proxy ./

ENTRYPOINT [ "./aws-sigv4-proxy" ]


FROM golang:1.17.7-alpine AS build

RUN apk --update add \
      ca-certificates \
      git

RUN mkdir /aws-sigv4-proxy
WORKDIR /aws-sigv4-proxy
COPY  . .

RUN go env -w GOPROXY=direct

RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/aws-sigv4-proxy

FROM alpine:latest
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /aws-sigv4-proxy ./

ENTRYPOINT [ "./aws-sigv4-proxy" ]


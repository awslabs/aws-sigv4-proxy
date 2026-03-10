FROM amazonlinux:2 AS build

RUN yum -y update && rm -rf /var/cache/yum/*
RUN yum install -y  \
      ca-certificates \
      git \
      bash \
      go

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/aws-sigv4-proxy .

FROM alpine:3.23

COPY --from=build /etc/ssl/certs/ca-bundle.crt /etc/ssl/certs/
COPY --from=build /bin/aws-sigv4-proxy /aws-sigv4-proxy
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

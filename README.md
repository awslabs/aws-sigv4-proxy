# AWS SigV4 Proxy

The AWS SigV4 Proxy will sign incoming HTTP requests and forward them to the host specified in the `Host` header.  

## Getting Started

Build and run the Proxy

```go
* The proxy will try to find credentials in the environment, shared credentials file, then the ec2 instance

docker build -t aws-sigv4-proxy .

# Env vars
docker run --rm -ti \
  -e 'AWS_ACCESS_KEY_ID=<YOUR ACCESS KEY ID>' \
  -e 'AWS_SECRET_ACCESS_KEY=<YOUR SECRET ACCESS KEY>' \
  -p 8080:8080 \
  aws-sigv4-proxy -v

***** or ******

# Shared Credentials
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v
```

## Examples

S3
```
# us-east-1
curl -s -H 'host: s3.us-west-2.amazonaws.com' http://localhost:8080/<BUCKET_NAME>

# other region
curl -s -H 'host: s3.<BUCKET_REGION>.amazonaws.com' http://localhost:8080/<BUCKET_NAME>
```

SQS
```sh
curl -s -H 'host: sqs.<AWS_REGION>.amazonaws.com' 'http://localhost:8080/<AWS_ACCOUNT_ID>/<QUEUE_NAME>?Action=SendMessage&MessageBody=example'
```

API Gateway
```sh
curl -H 'host: <REST_API_ID>.execute-api.<AWS_REGION>.amazonaws.com' http://localhost:8080/<STAGE>/<PATH>
```

## Reference

- [AWS SigV4 signing Docs ](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)


## License

This library is licensed under the Apache 2.0 License. 

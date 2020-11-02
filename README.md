# AWS SigV4 Proxy

The AWS SigV4 Proxy will sign incoming HTTP requests and forward them to the host specified in the `Host` header.

You can strip out arbirtary headers from the incoming request by using the -s option.

## Getting Started

Build and run the Proxy

```go
The proxy uses the default AWS SDK for Go credential search path:

* Environment variables.
* Shared credentials file.
* IAM role for Amazon EC2 or ECS task role

More information can be found in the [developer guide](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html)

docker build -t aws-sigv4-proxy .

# Env vars
docker run --rm -ti \
  -e 'AWS_ACCESS_KEY_ID=<YOUR ACCESS KEY ID>' \
  -e 'AWS_SECRET_ACCESS_KEY=<YOUR SECRET ACCESS KEY>' \
  -p 8080:8080 \
  aws-sigv4-proxy -v

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
curl -s -H 'host: s3.amazonaws.com' http://localhost:8080/<BUCKET_NAME>

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

Running the service and stripping out sigv2 authorization headers
```sh
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v -s Authorization
```

Running the service with Assume Role to use temporary credentials
```sh
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v --role-arn <ARN OF ROLE TO ASSUME>
```

## Reference

- [AWS SigV4 signing Docs ](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)


## License

This library is licensed under the Apache 2.0 License.

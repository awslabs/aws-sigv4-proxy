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
  -e 'AWS_SDK_LOAD_CONFIG=true' \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v
```

### Configuration

When running the Proxy, the following flags can be used (none are required) :
s", "
| Flag (or short form)          | Type     | Description                                                | Default |
|-------------------------------|----------|------------------------------------------------------------|---------|
| `verbose` or `v`              | Boolean  | Enable additional logging, implies all the log-* options   | `False` |
| `log-failed-requests`         | Boolean  | Log 4xx and 5xx response body                              | `False` |
| `log-signing-process`         | Boolean  | Log sigv4 signing process                                  | `False` |
| `unsigned-payload`            | Boolean  | Prevent signing of the payload"                            | `False` |
| `port`                        | String   | Port to serve http on                                      | `8080`  |
| `strip` or `s`                | String   | Headers to strip from incoming request                     | None    |
| `custom-headers`              | String   | Comma-separated list of custom headers in key=value format | None    |
| `duplicate-headers`           | String   | Duplicate headers to an X-Original- prefix name            | None    |
| `role-arn`                    | String   | Amazon Resource Name (ARN) of the role to assume           | None    |
| `name`                        | String   | AWS Service to sign for                                    | None    |
| `sign-host`                   | String   | Host to sign for                                           | None    |
| `host`                        | String   | Host to proxy to                                           | None    |
| `region`                      | String   | AWS region to sign for                                     | None    |
| `upstream-url-scheme`         | String   | Protocol to proxy with                                     | https   |
| `no-verify-ssl`               | Boolean  | Disable peer SSL certificate validation                    | `False` |
| `transport.idle-conn-timeout` | Duration | Idle timeout to the upstream service                       | `40s`   |

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
  -e 'AWS_SDK_LOAD_CONFIG=true' \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v -s Authorization
```

Running the service and preserving the original Authorization header as X-Original-Authorization (useful because Authorization header will be overwritten.)

```sh
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_SDK_LOAD_CONFIG=true' \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v --duplicate-headers Authorization
```

Running the service with Assume Role to use temporary credentials

```sh
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_SDK_LOAD_CONFIG=true' \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v --role-arn <ARN OF ROLE TO ASSUME>
```

Include service name & region overrides when you notice errors like `unable to determine service from host` for API gateway, for example.

```sh
docker run --rm -ti \
  -v ~/.aws:/root/.aws \
  -p 8080:8080 \
  -e 'AWS_SDK_LOAD_CONFIG=true' \
  -e 'AWS_PROFILE=<SOME PROFILE>' \
  aws-sigv4-proxy -v --name execute-api --region us-east-1
```

OpenSearch

* Access AWS OpenSearch domain, hosted in private subnet of AWS VPC, with access policy restricted to IAM role.

  Prepare connection (assume role, export `AWS_PROFILE`, run ssh tunnel):
    ```sh
    aws sts assume-role \
     --role-arn "arn:aws:iam::123456789012:role/example-role" \
     --role-session-name role-profile
    export AWS_PROFILE=role-profile
    
    ssh \
     -4 \
     -o BatchMode="yes" \
     -o StrictHostKeyChecking="no" \
     -o ProxyCommand="aws ssm start-session --target %h --region eu-west-1 --document-name AWS-StartSSHSession --parameters portNumber=%p" \
     -i /Users/user/.ssh/id_rsa ubuntu@i-bastion-host \
     -L 4443:vpc-private-domain-name.eu-west-1.es.amazonaws.com:443 \
     -N
    ```

    Finally, run proxy: 
    ```sh
    docker run --rm -ti \
         -v ~/.aws:/root/.aws \
         --network=bridge \
         -p 8080:8080 \
         -e "AWS_SDK_LOAD_CONFIG=true" \
         -e "AWS_PROFILE=role-profile" \
         aws-sigv4-proxy \
                 --verbose --log-failed-requests --log-signing-process --no-verify-ssl \
                 --name es --region eu-west-1 \
                 --host host.docker.internal:4443 \
                 --sign-host eu-west-1.es.amazonaws.com
    ```
  
  Access dashboard via http://localhost:8080/_dashboards/app/home#/tutorial_directory

## Reference

- [AWS SigV4 Signing Docs ](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)
- [AWS SigV4 Admission Controller](https://github.com/aws-observability/aws-sigv4-proxy-admission-controller) - Used to install the AWS SigV4 Proxy as a sidecar

## License

This library is licensed under the Apache 2.0 License.

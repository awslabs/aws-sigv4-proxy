#!/bin/bash

IMAGE=$1
CONTAINER_NAME="integ-test-$(date +%s)"

docker run -dt -p 8080:8080 \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_SESSION_TOKEN \
  -e AWS_REGION=us-east-1 \
  --name $CONTAINER_NAME \
  $IMAGE

# Wait for the container to start
sleep 5

curl -s -H 'host: s3.amazonaws.com' http://127.0.0.1:8080 | grep -o "<ListAllMyBucketsResult"
result=$?

docker stop $CONTAINER_NAME && docker rm $CONTAINER_NAME

if [[ "$result" == "0" ]]; then
  echo "Integration tests succeeded"
  exit 0
fi

echo "Integration tests failed"

exit 1

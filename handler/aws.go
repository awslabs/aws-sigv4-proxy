/*
 * Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License").
 * You may not use this file except in compliance with the License.
 * A copy of the License is located at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
 * express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package handler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

var services = map[string]*endpoints.ResolvedEndpoint{}

func init() {
	// Triple nested loop - 😭
	for _, partition := range endpoints.DefaultPartitions() {

		for _, service := range partition.Services() {
			for _, endpoint := range service.Endpoints() {
				resolvedEndpoint, _ := endpoint.ResolveEndpoint()
				host := strings.Replace(resolvedEndpoint.URL, "https://", "", 1)
				services[host] = &resolvedEndpoint
			}
		}
	}
}

var apiGatewayRegex = regexp.MustCompile(`^(?P<prefix>[a-zA-Z0-9_-]+)\.execute-api\.(?P<region>[a-zA-Z0-9_-]+)\.amazonaws\.com$`)
var elasticSearchRegex = regexp.MustCompile(`^(?P<prefix>[a-zA-Z0-9_-]+)\.(?P<region>[a-zA-Z0-9_-]+)\.es\.amazonaws\.com$`)

func determineAWSServiceFromHost(host string) *endpoints.ResolvedEndpoint {
	// handle api gateway endpoints
	apiGatewayMatches := apiGatewayRegex.FindStringSubmatch(host)
	if len(apiGatewayMatches) == 3 {
		return &endpoints.ResolvedEndpoint{
			URL:           fmt.Sprintf("https://%s", host),
			SigningMethod: "v4",
			SigningRegion: apiGatewayMatches[2],
			SigningName:   "execute-api",
			PartitionID:   "aws",
		}
	}
	// handle elasticsearch (OpenSearch) endpoints
	elasticSearchMatches := elasticSearchRegex.FindStringSubmatch(host)
	if len(elasticSearchMatches) == 3 {
		return &endpoints.ResolvedEndpoint{
			URL:           fmt.Sprintf("https://%s", host),
			SigningMethod: "v4",
			SigningRegion: elasticSearchMatches[2],
			SigningName:   "es",
			PartitionID:   "aws",
		}
	}

	endpoint := services[host]
	return endpoint
}

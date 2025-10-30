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
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

var services = map[string]endpoints.ResolvedEndpoint{}

func init() {
	// Triple nested loop - 😭
	for _, partition := range endpoints.DefaultPartitions() {

		for _, service := range partition.Services() {
			for _, endpoint := range service.Endpoints() {
				resolvedEndpoint, _ := endpoint.ResolveEndpoint()
				host := strings.Replace(resolvedEndpoint.URL, "https://", "", 1)
				services[host] = resolvedEndpoint
			}
		}
	}

	// Add api gateway endpoints
	for region := range endpoints.AwsPartition().Regions() {
		host := fmt.Sprintf("execute-api.%s.amazonaws.com", region)
		services[host] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", host), SigningMethod: "v4", SigningRegion: region, SigningName: "execute-api", PartitionID: "aws"}
	}
	// Add elasticsearch endpoints
	for region := range endpoints.AwsPartition().Regions() {
		host := fmt.Sprintf("%s.es.amazonaws.com", region)
		services[host] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", host), SigningMethod: "v4", SigningRegion: region, SigningName: "es", PartitionID: "aws"}
	}
	// Add managed prometheus + workspace endpoints
	for region := range endpoints.AwsPartition().Regions() {
		hostAps := fmt.Sprintf("aps.%s.amazonaws.com", region)
		services[hostAps] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", hostAps), SigningMethod: "v4", SigningRegion: region, SigningName: "aps", PartitionID: "aws"}

		hostApsws := fmt.Sprintf("aps-workspaces.%s.amazonaws.com", region)
		services[hostApsws] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", hostApsws), SigningMethod: "v4", SigningRegion: region, SigningName: "aps", PartitionID: "aws"}
	}
}

func determineAWSServiceFromHost(host string) *endpoints.ResolvedEndpoint {
	for endpoint, service := range services {
		if host == endpoint || (endpoint != "" && strings.HasSuffix(host, "."+endpoint)) {
			return &service
		}
	}
	return nil
}

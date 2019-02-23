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
		services[host] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", host), SigningMethod: "v4", SigningRegion: region, SigningName: "execute-api"}
	}

	// Add s3 endpoints
	for region := range endpoints.AwsPartition().Regions() {
		formats := []string{"s3-%s.amazonaws.com", "s3.%s.amazonaws.com"}
		for _, format := range formats {
			host := fmt.Sprintf(format, region)
			services[host] = endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", host), SigningMethod: "v4", SigningRegion: region, SigningName: "s3"}
		}
	}
}

func determineAWSServiceFromHost(host string) *endpoints.ResolvedEndpoint {
	for endpoint, service := range services {
		if strings.Contains(host, endpoint) {
			return &service
		}
	}

	return nil
}

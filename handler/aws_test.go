package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineSTSServiceFromHost(t *testing.T) {
	for _, tc := range []struct {
		host   string
		region string
	}{
		{"sts.eu-central-1.amazonaws.com", "eu-central-1"},
		{"sts.us-east-1.amazonaws.com", "us-east-1"},
		{"sts.amazonaws.com", "us-east-1"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			service := determineAWSServiceFromHost(tc.host)
			if service == nil {
				t.Fatal("STS endpoint not found")
			}
			assert.Equal(t, "sts", service.SigningName)
			assert.Equal(t, tc.region, service.SigningRegion)
			assert.Equal(t, "v4", service.SigningMethod)
			assert.Equal(t, "https://"+tc.host, service.URL)
		})
	}
}

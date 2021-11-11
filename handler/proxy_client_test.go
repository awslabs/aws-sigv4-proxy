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
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
)

type mockHTTPClient struct {
	Client
	Request *http.Request
	Fail    bool
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.Fail {
		return nil, fmt.Errorf("mockHTTPClient.Do failed")
	}
	m.Request = req
	return &http.Response{}, nil
}

type mockProvider struct {
	credentials.Provider
	Fail bool
}

func (m *mockProvider) Retrieve() (credentials.Value, error) {
	if m.Fail {
		return credentials.Value{}, fmt.Errorf("mockProvider.Retrieve failed")
	}
	return credentials.Value{}, nil
}

func TestProxyClient_Do(t *testing.T) {
	type want struct {
		resp    *http.Response
		request *http.Request
		service *endpoints.ResolvedEndpoint
		err     error
	}

	tests := []struct {
		name        string
		request     *http.Request
		proxyClient *ProxyClient
		want        *want
	}{
		{
			name: "should fail if unable to build new request",
			request: &http.Request{
				Method: "💩💩💩💩💩",
				URL:    &url.URL{},
				Host:   "ec2.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`net/http: invalid method "💩💩💩💩💩"`),
			},
		},
		{
			name: "should fail if unable to determine service name",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`unable to determine service from host: badservice.host`),
			},
		},
		{
			name: "should use SignNameOverride and RegionOverride if provided",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client:              &mockHTTPClient{},
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "badservice.host",
				},
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://badservice.host",
					PartitionID:        "",
					SigningRegion:      "us-west-2",
					SigningName:        "ec2",
					SigningNameDerived: false,
					SigningMethod:      "v4",
				},
			},
		},
		{
			name: "should use HostOverride if provided",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client:              &mockHTTPClient{},
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
				HostOverride:        "host.override",
			},
			want: &want{
				resp:    &http.Response{},
				request: &http.Request{Host: "host.override"},
				err:     nil,
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://host.override",
					PartitionID:        "",
					SigningRegion:      "us-west-2",
					SigningName:        "ec2",
					SigningNameDerived: false,
					SigningMethod:      "v4",
				},
			},
		},
		{
			name: "should fail if unable to sign request",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "monitoring.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`mockProvider.Retrieve failed`),
			},
		},
		{
			name: "should fail if unable to sign request",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "monitoring.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp:    nil,
				request: nil,
				err:     fmt.Errorf(`mockProvider.Retrieve failed`),
			},
		},
		{
			name: "should fail if request to target host fails",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "monitoring.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{
					Fail: true,
				},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`mockHTTPClient.Do failed`),
			},
		},
		{
			name: "should return request when everything 👍 (presign codepath)",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "s3.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "s3.amazonaws.com",
				},
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://s3.amazonaws.com",
					PartitionID:        "aws",
					SigningRegion:      "us-east-1",
					SigningName:        "s3",
					SigningNameDerived: true,
					SigningMethod:      "s3",
				},
			},
		},
		{
			name: "should return request when everything 👍",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "monitoring.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "monitoring.us-west-2.amazonaws.com",
				},
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://monitoring.us-west-2.amazonaws.com",
					PartitionID:        "aws",
					SigningRegion:      "us-west-2",
					SigningName:        "monitoring",
					SigningNameDerived: true,
					SigningMethod:      "v4",
				},
			},
		},
		{
			name: "should return request when everything 👍 (api gateway)",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "abcdef1234.execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "abcdef1234.execute-api.us-west-2.amazonaws.com",
				},
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://abcdef1234.execute-api.us-west-2.amazonaws.com",
					PartitionID:        "aws",
					SigningRegion:      "us-west-2",
					SigningName:        "execute-api",
					SigningNameDerived: false,
					SigningMethod:      "v4",
				},
			},
		},
		{
			name: "should return request when everything 👍 (es)",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "test-abcdefghi.us-west-2.es.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "test-abcdefghi.us-west-2.es.amazonaws.com",
				},
				service: &endpoints.ResolvedEndpoint{
					URL:                "https://test-abcdefghi.us-west-2.es.amazonaws.com",
					PartitionID:        "aws",
					SigningRegion:      "us-west-2",
					SigningName:        "es",
					SigningNameDerived: false,
					SigningMethod:      "v4",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err :=
				tt.proxyClient.Do(tt.request)

			assert.Equal(t, tt.want.resp, resp)
			assert.Equal(t, tt.want.err, err)

			if err == nil {
				service := tt.proxyClient.serviceForURL(tt.proxyClient.proxyURLForRequest(tt.request))
				assert.Equal(t, tt.want.service, service)
			}

			assert.True(t, verifyRequest(tt.proxyClient.Client.(*mockHTTPClient).Request, tt.want.request))
		})
	}
}

func verifyRequest(received *http.Request, expected *http.Request) bool {
	if expected == nil {
		return received == nil
	}

	return received.Host == expected.Host
}

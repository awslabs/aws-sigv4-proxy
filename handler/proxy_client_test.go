package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
)

type mockHTTPClient struct {
	Client
	Fail bool
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.Fail {
		return nil, fmt.Errorf("mockHTTPClient.Do failed")
	}
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
		resp *http.Response
		err  error
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
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{},
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
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Region: "us-west-2",
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
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Region: "us-west-2",
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
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Region: "us-west-2",
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`mockProvider.Retrieve failed`),
			},
		},
		{
			name: "should fail if request to target host fails",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Region: "us-west-2",
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
			name: "should return request when everything 👍 (s3/presign codepath)",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "s3.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				S3Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Region: "us-west-2",
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
			},
		},
		{
			name: "should return request when everything 👍",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Region: "us-west-2",
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// resp, err :=
			tt.proxyClient.Do(tt.request)

			// assert.Equal(t, tt.want.resp, resp)
			// assert.Equal(t, tt.want.err, err)
		})
	}
}

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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockProxyClient struct {
	Fail     bool
	Response *http.Response
}

func (m *mockProxyClient) Do(req *http.Request) (*http.Response, error) {
	if m.Fail {
		return nil, fmt.Errorf("mockProxyClient.Do failed")
	}

	return m.Response, nil
}
func TestHandler_ServeHTTP(t *testing.T) {
	type want struct {
		statusCode int
		header     http.Header
		body       []byte
	}

	tests := []struct {
		name    string
		request *http.Request
		handler *Handler
		want    *want
	}{
		{
			name: "responds with 502 if proxy request fails",
			handler: &Handler{
				ProxyClients: map[string]Client{"default": &mockProxyClient{Fail: true}},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusBadGateway,
				body:       []byte(`unable to proxy request - mockProxyClient.Do failed`),
				header:     http.Header{},
			},
		},
		{
			name: "responds with proxied response if everything is 👍",
			handler: &Handler{
				ProxyClients: map[string]Client{
					"default": &mockProxyClient{
						Response: &http.Response{
							Header: http.Header{
								"test": []string{"header"},
							},
							Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`proxy call successful`))),
						},
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Test": []string{"header"},
				},
				body: []byte(`proxy call successful`),
			},
		},
		{
			name: "uses host config-set if a match is found",
			handler: &Handler{
				ProxyClients: map[string]Client{
					"default":  &mockProxyClient{Fail: true},
					"example.com": &mockProxyClient{
						Response: &http.Response{
							Header: http.Header{
								"test": []string{"this came from the target host"},
							},
							Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`proxy call successful`))),
						},
					},
				},
			},
			request: &http.Request{Host: "example.com"},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Test": []string{"this came from the target host"},
				},
				body: []byte(`proxy call successful`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRecorder()

			tt.handler.ServeHTTP(r, tt.request)

			response := r.Result()
			responseBody, _ := ioutil.ReadAll(response.Body)

			assert.Equal(t, tt.want.statusCode, response.StatusCode)
			assert.Equal(t, tt.want.header, response.Header)
			assert.Equal(t, tt.want.body, responseBody)

			response.Body.Close()
		})
	}
}

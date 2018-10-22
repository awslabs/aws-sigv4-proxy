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
			name: "responds with 400 if proxy request fails",
			handler: &Handler{
				ProxyClient: &mockProxyClient{Fail: true},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusBadRequest,
				body:       []byte(`mockProxyClient.Do failed`),
				header:     http.Header{},
			},
		},
		{
			name: "responds with proxied response if everything is 👍",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						Header: http.Header{
							"test": []string{"header"},
						},
						Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`proxy call successful`))),
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

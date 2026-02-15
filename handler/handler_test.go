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
	"io"
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

// chunkReader simulates reading data in multiple chunks
type chunkReader struct {
	chunks [][]byte
	index  int
}

func newChunkReader(chunks [][]byte) *chunkReader {
	return &chunkReader{
		chunks: chunks,
		index:  0,
	}
}

func (cr *chunkReader) Read(p []byte) (n int, err error) {
	if cr.index >= len(cr.chunks) {
		return 0, io.EOF
	}

	chunk := cr.chunks[cr.index]
	cr.index++

	n = copy(p, chunk)
	return n, nil
}

func (cr *chunkReader) Close() error {
	return nil
}

// captureFlushWriter is a ResponseWriter that captures writes and flushes
type captureFlushWriter struct {
	header      http.Header
	statusCode  int
	writes      [][]byte // Each write operation captured
	flushes     int      // Number of times Flush was called
	wroteHeader bool
}

func newCaptureFlushWriter() *captureFlushWriter {
	return &captureFlushWriter{
		header: make(http.Header),
		writes: make([][]byte, 0),
	}
}

func (c *captureFlushWriter) Header() http.Header {
	return c.header
}

func (c *captureFlushWriter) Write(data []byte) (int, error) {
	// Make a copy to avoid issues with buffer reuse
	chunk := make([]byte, len(data))
	copy(chunk, data)
	c.writes = append(c.writes, chunk)
	return len(data), nil
}

func (c *captureFlushWriter) WriteHeader(statusCode int) {
	if !c.wroteHeader {
		c.statusCode = statusCode
		c.wroteHeader = true
	}
}

func (c *captureFlushWriter) Flush() {
	c.flushes++
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
				ProxyClient: &mockProxyClient{Fail: true},
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
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
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
		{
			name: "handles SSE response with single chunk",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type":      []string{"text/event-stream"},
							"Transfer-Encoding": []string{"chunked"},
							"Cache-Control":     []string{"no-cache"},
						},
						TransferEncoding: []string{"chunked"},
						Body: ioutil.NopCloser(bytes.NewBuffer([]byte(
							"data: event 1\n\n" +
								"data: event 2\n\n" +
								"data: event 3\n\n",
						))),
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Content-Type":      []string{"text/event-stream"},
					"Transfer-Encoding": []string{"chunked"},
					"Cache-Control":     []string{"no-cache"},
				},
				body: []byte("data: event 1\n\ndata: event 2\n\ndata: event 3\n\n"),
			},
		},
		{
			name: "handles SSE response sent in multiple chunks",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type":      []string{"text/event-stream"},
							"Transfer-Encoding": []string{"chunked"},
							"Cache-Control":     []string{"no-cache"},
						},
						TransferEncoding: []string{"chunked"},
						Body: newChunkReader([][]byte{
							[]byte("data: event 1\n\n"),
							[]byte("data: event 2\n\n"),
							[]byte("data: event 3\n\n"),
							[]byte("id: 100\nevent: update\ndata: event 4\n\n"),
							[]byte("data: event 5\n\n"),
						}),
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Content-Type":      []string{"text/event-stream"},
					"Transfer-Encoding": []string{"chunked"},
					"Cache-Control":     []string{"no-cache"},
				},
				body: []byte("data: event 1\n\ndata: event 2\n\ndata: event 3\n\nid: 100\nevent: update\ndata: event 4\n\ndata: event 5\n\n"),
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

func TestHandler_StreamsChunksIncrementally(t *testing.T) {
	// Test that verifies chunks are actually streamed incrementally with flushes
	// rather than being buffered and sent all at once
	type want struct {
		statusCode     int
		header         http.Header
		body           []byte
		expectedWrites int
		expectedChunks [][]byte
	}

	tests := []struct {
		name    string
		request *http.Request
		handler *Handler
		want    *want
	}{
		{
			name: "streams chunks with flushes between writes",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type":      []string{"text/plain"},
							"Transfer-Encoding": []string{"chunked"},
						},
						TransferEncoding: []string{"chunked"},
						ContentLength:    -1,
						Body: newChunkReader([][]byte{
							[]byte("chunk 1\n"),
							[]byte("chunk 2\n"),
							[]byte("chunk 3\n"),
							[]byte("chunk 4\n"),
						}),
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Content-Type":      []string{"text/plain"},
					"Transfer-Encoding": []string{"chunked"},
				},
				body:           []byte("chunk 1\nchunk 2\nchunk 3\nchunk 4\n"),
				expectedWrites: 4,
				expectedChunks: [][]byte{
					[]byte("chunk 1\n"),
					[]byte("chunk 2\n"),
					[]byte("chunk 3\n"),
					[]byte("chunk 4\n"),
				},
			},
		},
		{
			name: "streams SSE events incrementally",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type":      []string{"text/event-stream"},
							"Cache-Control":     []string{"no-cache"},
							"Transfer-Encoding": []string{"chunked"},
						},
						TransferEncoding: []string{"chunked"},
						ContentLength:    -1,
						Body: newChunkReader([][]byte{
							[]byte("data: event 1\n\n"),
							[]byte("data: event 2\n\n"),
							[]byte("data: event 3\n\n"),
						}),
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Content-Type":      []string{"text/event-stream"},
					"Cache-Control":     []string{"no-cache"},
					"Transfer-Encoding": []string{"chunked"},
				},
				body:           []byte("data: event 1\n\ndata: event 2\n\ndata: event 3\n\n"),
				expectedWrites: 3,
				expectedChunks: [][]byte{
					[]byte("data: event 1\n\n"),
					[]byte("data: event 2\n\n"),
					[]byte("data: event 3\n\n"),
				},
			},
		},
		{
			name: "handles streaming with single large chunk",
			handler: &Handler{
				ProxyClient: &mockProxyClient{
					Response: &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type":      []string{"application/octet-stream"},
							"Transfer-Encoding": []string{"chunked"},
						},
						TransferEncoding: []string{"chunked"},
						ContentLength:    -1,
						Body:             ioutil.NopCloser(bytes.NewReader(bytes.Repeat([]byte("abcdefghij"), 5000))), // 50KB
					},
				},
			},
			request: &http.Request{},
			want: &want{
				statusCode: http.StatusOK,
				header: http.Header{
					"Content-Type":      []string{"application/octet-stream"},
					"Transfer-Encoding": []string{"chunked"},
				},
				body:           bytes.Repeat([]byte("abcdefghij"), 5000), // 50KB
				expectedWrites: 2,                                        // Should be split by 32KB buffer
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newCaptureFlushWriter()

			tt.handler.ServeHTTP(w, tt.request)

			assert.Equal(t, tt.want.statusCode, w.statusCode)
			assert.Equal(t, tt.want.header, w.header)
			assert.Equal(t, tt.want.expectedWrites, len(w.writes))
			assert.Equal(t, len(w.writes), w.flushes)
			if tt.want.expectedChunks != nil {
				for i, expectedChunk := range tt.want.expectedChunks {
					assert.Equal(t, expectedChunk, w.writes[i])
				}
			}
			var fullBody bytes.Buffer
			for _, write := range w.writes {
				fullBody.Write(write)
			}
			assert.Equal(t, tt.want.body, fullBody.Bytes())
		})
	}
}

func TestHandler_BuffersKnownLengthResponses(t *testing.T) {
	// This test ensures we keep the legacy behavior where fixed-length responses
	// are buffered and emitted with a single write (preserving Content-Length)
	// instead of forcing chunked transfer downstream.
	body := []byte("chunk 1\nchunk 2\nchunk 3\nchunk 4\n")
	handler := &Handler{
		ProxyClient: &mockProxyClient{
			Response: &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
				ContentLength: int64(len(body)),
				Body:          ioutil.NopCloser(bytes.NewReader(body)),
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := newCaptureFlushWriter()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.statusCode)
	assert.Equal(t, 1, len(w.writes))
	assert.Equal(t, 0, w.flushes)
	assert.Equal(t, body, w.writes[0])
}

// failingWriter is a ResponseWriter that fails after a certain number of bytes
type failingWriter struct {
	header         http.Header
	statusCode     int
	bytesWritten   int
	failAfterBytes int
}

func (w *failingWriter) Header() http.Header {
	return w.header
}

func (w *failingWriter) Write(data []byte) (int, error) {
	if w.failAfterBytes >= 0 && w.bytesWritten+len(data) > w.failAfterBytes {
		remaining := w.failAfterBytes - w.bytesWritten
		if remaining > 0 {
			w.bytesWritten += remaining
			return remaining, io.ErrClosedPipe
		}
		return 0, io.ErrClosedPipe
	}
	w.bytesWritten += len(data)
	return len(data), nil
}

func (w *failingWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *failingWriter) Flush() {
}

// TestHandler_WriterFailure tests that the handler doesn't panic when the ResponseWriter
// fails during streaming. This regression test catches a bug where err.Error() was called
// on a nil error when a write failed but the read was successful.
func TestHandler_WriterFailure(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		failAfterBytes int
	}{
		{
			name:           "writer fails after partial write",
			body:           []byte("long data stream"),
			failAfterBytes: 5,
		},
		{
			name:           "writer fails immediately",
			body:           []byte("test data"),
			failAfterBytes: 0,
		},
		{
			name:           "writer fails mid-stream with large body",
			body:           bytes.Repeat([]byte("x"), 10000),
			failAfterBytes: 1000,
		},
		{
			name:           "writer fails after first chunk",
			body:           bytes.Repeat([]byte("y"), 50000), // Larger than 32KB buffer
			failAfterBytes: 32 * 1024,                        // Fail after first buffer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyClient := &mockProxyClient{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type":      []string{"text/plain"},
						"Transfer-Encoding": []string{"chunked"},
					},
					TransferEncoding: []string{"chunked"},
					ContentLength:    -1,
					Body:             ioutil.NopCloser(bytes.NewReader(tt.body)),
				},
			}

			handler := &Handler{
				ProxyClient: proxyClient,
			}

			req := httptest.NewRequest("GET", "http://example.com", nil)
			w := &failingWriter{
				header:         make(http.Header),
				failAfterBytes: tt.failAfterBytes,
			}

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.statusCode)
			if tt.failAfterBytes > 0 {
				assert.True(t, w.bytesWritten > 0)
			}
		})
	}
}

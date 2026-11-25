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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// zeroReader yields n zero bytes without allocating, and records the largest
// buffer it was asked to fill.
type zeroReader struct {
	remaining int64
	maxRead   int
}

func (z *zeroReader) Read(p []byte) (int, error) {
	if len(p) > z.maxRead {
		z.maxRead = len(p)
	}
	if z.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.remaining {
		n = z.remaining
	}
	clear(p[:n])
	z.remaining -= n
	return int(n), nil
}

func (z *zeroReader) Close() error { return nil }

// failingReader returns data, then err.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func (f *failingReader) Close() error { return nil }

// countingWriter discards the body and counts bytes.
type countingWriter struct {
	header http.Header
	status int
	n      int64
}

func (c *countingWriter) Header() http.Header { return c.header }
func (c *countingWriter) WriteHeader(s int)   { c.status = s }
func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

func newHandler(resp *http.Response, size int) *Handler {
	return &Handler{ProxyClient: &mockProxyClient{Response: resp}, BufferSize: size}
}

func TestServeHTTP_LargeBodyUsesBoundedMemory(t *testing.T) {
	const total = 1 << 30 // 1 GiB
	body := &zeroReader{remaining: total}
	h := newHandler(&http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}, 0)
	w := &countingWriter{header: http.Header{}}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	h.ServeHTTP(w, &http.Request{})
	runtime.ReadMemStats(&after)

	assert.Equal(t, int64(total), w.n)
	// Total allocation over the whole request stays far below the body size.
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(4<<20))
}

func TestServeHTTP_BufferSizeIsHonored(t *testing.T) {
	for _, size := range []int{0, 1000, 64 * 1024} {
		want := size
		if size == 0 {
			want = DefaultBufferSize
		}
		body := &zeroReader{remaining: 1 << 20}
		h := newHandler(&http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}, size)
		w := &countingWriter{header: http.Header{}}
		h.ServeHTTP(w, &http.Request{})
		assert.Equal(t, want, body.maxRead, "buffer size %d", size)
		assert.Equal(t, int64(1<<20), w.n)
	}
}

func TestServeHTTP_KeepsContentLengthWithoutChunking(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789abcdef"), 64*1024) // 1 MiB, many chunks
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Length": []string{strconv.Itoa(len(payload))},
			"Etag":           []string{`"abc"`},
		},
		Body: io.NopCloser(bytes.NewReader(payload)),
	}
	srv := httptest.NewServer(newHandler(upstreamResp, 1000))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	assert.NoError(t, err)
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.Equal(t, int64(len(payload)), resp.ContentLength)
	assert.Empty(t, resp.TransferEncoding)
	assert.Equal(t, `"abc"`, resp.Header.Get("Etag"))
	assert.True(t, bytes.Equal(payload, got))
}

func TestServeHTTP_PreservesStatusAndEmptyBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotModified, http.StatusNotFound} {
		resp := &http.Response{StatusCode: status, Header: http.Header{"X-Test": []string{"1"}}, Body: io.NopCloser(bytes.NewReader(nil))}
		rec := httptest.NewRecorder()
		newHandler(resp, 0).ServeHTTP(rec, &http.Request{})
		assert.Equal(t, status, rec.Code)
		assert.Equal(t, "1", rec.Header().Get("X-Test"))
		assert.Empty(t, rec.Body.Bytes())
	}
}

func TestServeHTTP_ErrorStatusBodyIsStreamed(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader([]byte("<Error>AccessDenied</Error>")))}
	rec := httptest.NewRecorder()
	newHandler(resp, 0).ServeHTTP(rec, &http.Request{})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "<Error>AccessDenied</Error>", rec.Body.String())
}

func TestServeHTTP_ErrorBeforeFirstByteReturns500(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: &failingReader{err: errors.New("connection reset")}}
	rec := httptest.NewRecorder()
	newHandler(resp, 0).ServeHTTP(rec, &http.Request{})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "connection reset")
}

func TestServeHTTP_ErrorMidStreamAbortsConnection(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 5000)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: &failingReader{data: data, err: errors.New("connection reset")}}
	rec := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		newHandler(resp, 1000).ServeHTTP(rec, &http.Request{})
	}()

	assert.Equal(t, http.ErrAbortHandler, recovered)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, data, rec.Body.Bytes()) // partial data was delivered before the abort
}

func TestServeHTTP_MidStreamFailureIsSeenAsTruncatedByClient(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 5000)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Length": []string{"10000"}},
		Body:       &failingReader{data: data, err: errors.New("connection reset")},
	}
	srv := httptest.NewServer(newHandler(resp, 1000))
	defer srv.Close()

	r, err := http.Get(srv.URL)
	assert.NoError(t, err)
	defer r.Body.Close()
	_, err = io.ReadAll(r.Body)
	assert.Error(t, err) // unexpected EOF, not a clean completion
}

func TestServeHTTP_EventStreamIsFlushed(t *testing.T) {
	stream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader([]byte("data: hi\n\n"))),
	}
	rec := httptest.NewRecorder()
	newHandler(stream, 0).ServeHTTP(rec, &http.Request{})
	assert.True(t, rec.Flushed)

	plain := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
		Body:       io.NopCloser(bytes.NewReader([]byte("data"))),
	}
	rec = httptest.NewRecorder()
	newHandler(plain, 0).ServeHTTP(rec, &http.Request{})
	assert.False(t, rec.Flushed)
}

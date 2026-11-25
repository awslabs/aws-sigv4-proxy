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
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// DefaultBufferSize is the response buffer size used when Handler.BufferSize is unset.
const DefaultBufferSize = 256 * 1024

type Handler struct {
	ProxyClient Client

	// BufferSize is the size in bytes of the buffer used to stream each
	// response body to the client. Memory use per request is bounded by it,
	// regardless of the response size. Defaults to DefaultBufferSize.
	BufferSize int
}

func (h *Handler) write(w http.ResponseWriter, status int, body []byte) {
	w.WriteHeader(status)
	w.Write(body)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := h.ProxyClient.Do(r)
	if err != nil {
		errorMsg := "unable to proxy request"
		log.WithError(err).Error(errorMsg)
		h.write(w, http.StatusBadGateway, []byte(fmt.Sprintf("%v - %v", errorMsg, err.Error())))
		return
	}
	defer resp.Body.Close()

	size := h.BufferSize
	if size <= 0 {
		size = DefaultBufferSize
	}
	buf := make([]byte, size)

	// Read the first chunk before sending headers, so a failure before any
	// data arrives can still be reported with an error status.
	n, err := resp.Body.Read(buf)
	for n == 0 && err == nil {
		n, err = resp.Body.Read(buf)
	}
	if n == 0 && err != io.EOF {
		errorMsg := "error while reading response from upstream"
		log.WithError(err).Error(errorMsg)
		h.write(w, http.StatusInternalServerError, []byte(fmt.Sprintf("%v - %v", errorMsg, err.Error())))
		return
	}

	// copy headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Event streams must be delivered as they arrive.
	var flusher http.Flusher
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		flusher, _ = w.(http.Flusher)
	}

	// Stream the rest of the body through the same buffer.
	for {
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				log.WithError(werr).Error("error while writing response to client")
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			// Headers are already sent. Abort the connection so the client
			// sees a truncated response instead of a seemingly complete one.
			log.WithError(err).Error("error while streaming response from upstream")
			panic(http.ErrAbortHandler)
		}
		n, err = resp.Body.Read(buf)
	}
}

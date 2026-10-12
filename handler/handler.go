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

	log "github.com/sirupsen/logrus"
)

type Handler struct {
	ProxyClient Client
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

	respChunked := chunked(resp.TransferEncoding)
	shouldStream := respChunked || resp.ContentLength < 0

	// copy headers first, before writing status
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	if !shouldStream {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errorMsg := "error while reading response from upstream"
			log.WithError(err).Error(errorMsg)
			h.write(w, http.StatusInternalServerError, []byte(fmt.Sprintf("%v - %v", errorMsg, err.Error())))
			return
		}

		h.write(w, resp.StatusCode, body)
		return
	}

	// write status code for streaming responses so downstream knows we started sending data
	w.WriteHeader(resp.StatusCode)

	// stream response body directly to the client with explicit flushing
	// Use a smaller buffer and flush after each chunk for true streaming
	flusher, canFlush := w.(http.Flusher)
	// initialize buffer of 32K bytes
	buf := make([]byte, 32*1024)
	errorMsg := "error while streaming response from upstream"

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				log.WithError(writeErr).Error(errorMsg)
				// Don't try to write to w if it's already failing
				return
			}
			// Flush after each chunk to ensure immediate delivery
			if canFlush {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithError(err).Error(errorMsg)
			// Try to write error to response, but don't panic if it fails
			fmt.Fprintf(w, "%v - %v", errorMsg, err.Error())
			return
		}
	}
}

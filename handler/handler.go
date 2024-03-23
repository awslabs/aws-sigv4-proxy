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
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		return
	} else if err != nil {
	    errorMsg := "unable to proxy request"
		log.WithError(err).Error(errorMsg)
		h.write(w, http.StatusBadGateway, []byte(fmt.Sprintf("%v - %v", errorMsg, err.Error())))
		return
	}
	defer resp.Body.Close()

	// read response body
	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, resp.Body); err != nil {
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

	h.write(w, resp.StatusCode, buf.Bytes())
}

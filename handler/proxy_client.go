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
	"net/http/httputil"
	"time"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	log "github.com/sirupsen/logrus"
)

// Client is an interface to make testing http.Client calls easier
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// ProxyClient implements the Client interface
type ProxyClient struct {
	Signer                  *v4.Signer
	Client                  Client
	StripRequestHeaders     []string
	CustomHeaders           http.Header
	DuplicateRequestHeaders []string
	SigningNameOverride     string
	SigningHostOverride     string
	HostOverride            string
	RegionOverride          string
	LogFailedRequest        bool
	SchemeOverride          string
	RateLimiter 			*RateLimiter
}

func (p *ProxyClient) sign(req *http.Request, service *endpoints.ResolvedEndpoint) error {
	body := bytes.NewReader([]byte{})

	if req.Body != nil {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}

		body = bytes.NewReader(b)
	}

	// S3 service should not have any escaping applied.
	// https://github.com/aws/aws-sdk-go/blob/main/aws/signer/v4/v4.go#L467-L470
	if service.SigningName == "s3" {
		p.Signer.DisableURIPathEscaping = true

		// Enable URI escaping for subsequent calls.
		defer func() {
			p.Signer.DisableURIPathEscaping = false
		}()
	}

	var err error
	switch service.SigningMethod {
	case "v4", "s3v4":
		_, err = p.Signer.Sign(req, body, service.SigningName, service.SigningRegion, time.Now())
		break
	case "s3":
		_, err = p.Signer.Presign(req, body, service.SigningName, service.SigningRegion, time.Duration(time.Hour), time.Now())
		break
	default:
		err = fmt.Errorf("unable to sign with specified signing method %s for service %s", service.SigningMethod, service.SigningName)
		break
	}

	if err == nil {
		log.WithFields(log.Fields{"service": service.SigningName, "region": service.SigningRegion}).Debug("signed request")
	}

	return err
}

func copyHeaderWithoutOverwrite(dst, src http.Header) {
	for k, vv := range src {
		if _, ok := dst[k]; !ok {
			for _, v := range vv {
				dst.Add(k, v)
			}
		}
	}
}

// RFC2616, Section 4.4: If a Transfer-Encoding header field (Section 14.41) is
// present and has any value other than "identity", then the transfer-length is
// defined by use of the "chunked" transfer-coding (Section 3.6). [...] If a
// message is received with both a Transfer-Encoding header field and a
// Content-Length header field, the latter MUST be ignored.
//
// RFC2616, Section 3.6: Whenever a transfer-coding is applied to a
// message-body, the set of transfer-codings MUST include "chunked", unless the
// message is terminated by closing the connection.
func chunked(transferEncoding []string) bool {
	for _, v := range transferEncoding {
		// This interprets identity-only headers as no header.
		if v != "identity" {
			return true
		}
	}
	return false
}

func readDownStreamRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}
	defer req.Body.Close()
	return io.ReadAll(req.Body)
}

func (p *ProxyClient) Do(req *http.Request) (*http.Response, error) {
	// Add rate limiting check at the start of the Do method
	if p.RateLimiter != nil && !p.RateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	proxyURL := *req.URL
	if p.HostOverride != "" {
		proxyURL.Host = p.HostOverride

	} else {
		proxyURL.Host = req.Host
	}
	proxyURL.Scheme = "https"
	if p.SchemeOverride != "" {
		proxyURL.Scheme = p.SchemeOverride
	}

	if log.GetLevel() == log.DebugLevel {
		initialReqDump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.WithError(err).Error("unable to dump request")
		}
		log.WithField("request", string(initialReqDump)).Debug("Initial request dump:")
	}

	// Save the request body into memory so that it's rewindable during retry.
	// See https://github.com/awslabs/aws-sigv4-proxy/issues/185
	// This may increase memory demand, but the demand should be ok for most cases. If there
	// are cases proven to be very problematic, we can consider adding a flag to disable this.
	proxyReqBody, err := readDownStreamRequestBody(req)
	if err != nil {
		return nil, err
	}

	proxyReq, err := http.NewRequest(req.Method, proxyURL.String(), bytes.NewReader(proxyReqBody))
	if err != nil {
		return nil, err
	}

	var reqChunked = chunked(req.TransferEncoding)

	// Ignore ContentLength if "chunked" transfer-coding is used.
	if !reqChunked && req.ContentLength >= 0 {
		proxyReq.ContentLength = req.ContentLength
	}

	var service *endpoints.ResolvedEndpoint
	if p.SigningHostOverride != "" {
		proxyReq.Host = p.SigningHostOverride
	}
	if p.SigningNameOverride != "" && p.RegionOverride != "" {
		service = &endpoints.ResolvedEndpoint{URL: fmt.Sprintf("%s://%s", proxyURL.Scheme, proxyURL.Host), SigningMethod: "v4", SigningRegion: p.RegionOverride, SigningName: p.SigningNameOverride}
	} else {
		service = determineAWSServiceFromHost(req.Host)
	}
	if service == nil {
		return nil, fmt.Errorf("unable to determine service from host: %s", req.Host)
	}

	if err := p.sign(proxyReq, service); err != nil {
		return nil, err
	}

	// go Documentation net/http, func (*Request) Write: If Body is present,
	// Content-Length is <= 0 and TransferEncoding hasn't been set to
	// "identity", Write adds "Transfer-Encoding: chunked" to the header.
	// Body is closed after it is sent.
	//
	// Service like S3 does not support chunk encoding. We need to manipulate
	// the Body value after signv4 signing because the signing process wraps the
	// original body into another struct, which will result in
	// Transfer-Encoding: chunked being set.
	if !reqChunked {
		// Set to identity to prevent write() from setting it to chunked.
		proxyReq.TransferEncoding = []string{"identity"}
	} else {
		proxyReq.TransferEncoding = req.TransferEncoding
	}

	// Remove any headers specified
	for _, header := range p.StripRequestHeaders {
		log.WithField("StripHeader", string(header)).Debug("Stripping Header:")
		req.Header.Del(header)
	}

	// Duplicate the header value for any headers specified into a new header
	// with an "X-Original-" prefix.
	for _, header := range p.DuplicateRequestHeaders {
		headerValue := req.Header.Get(header)
		if headerValue == "" {
			log.WithField("DuplicateHeader", string(header)).Debug("Header empty, will not duplicate:")
			continue
		}

		log.WithField("DuplicateHeader", string(header)).Debug("Duplicate Header to X-Original-* Prefix:")
		newHeaderName := fmt.Sprintf("X-Original-%s", header)
		proxyReq.Header.Set(newHeaderName, headerValue)
	}

	// Add origin headers after request is signed (no overwrite)
	copyHeaderWithoutOverwrite(proxyReq.Header, req.Header)

	// Add custom headers (no overwrite)
	copyHeaderWithoutOverwrite(proxyReq.Header, p.CustomHeaders)

	if log.GetLevel() == log.DebugLevel {
		proxyReqDump, err := httputil.DumpRequest(proxyReq, true)
		if err != nil {
			log.WithError(err).Error("unable to dump request")
		}
		log.WithField("request", string(proxyReqDump)).Debug("proxying request")
	}

	resp, err := p.Client.Do(proxyReq)
	if err != nil {
		return nil, err
	}

	if (p.LogFailedRequest || log.GetLevel() == log.DebugLevel) && resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		log.WithField("request", fmt.Sprintf("%s %s", proxyReq.Method, proxyReq.URL)).
			WithField("status_code", resp.StatusCode).
			WithField("message", string(b)).
			Error("error proxying request")

		// Need to "reset" the response body because we consumed the stream above, otherwise caller will
		// get empty body.
		resp.Body = io.NopCloser(bytes.NewBuffer(b))
	}

	return resp, nil
}

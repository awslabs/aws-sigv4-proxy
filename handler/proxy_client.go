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
	Signer              *v4.Signer
	Client              Client
	StripRequestHeaders []string
	SigningNameOverride string
	HostOverride        string
	RegionOverride      string
	LogFailedRequest    bool
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
        if (service.SigningName == "s3") {
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

func (p *ProxyClient) Do(req *http.Request) (*http.Response, error) {
	proxyURL := *req.URL
	if p.HostOverride != "" {
		proxyURL.Host = p.HostOverride

	} else {
		proxyURL.Host = req.Host
	}
	proxyURL.Scheme = "https"

	if log.GetLevel() == log.DebugLevel {
		initialReqDump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.WithError(err).Error("unable to dump request")
		}
		log.WithField("request", string(initialReqDump)).Debug("Initial request dump:")
	}

	proxyReq, err := http.NewRequest(req.Method, proxyURL.String(), req.Body)
	if err != nil {
		return nil, err
	}
	if req.ContentLength >= 0 {
		proxyReq.ContentLength = req.ContentLength
	}

	var service *endpoints.ResolvedEndpoint
	if p.SigningNameOverride != "" && p.RegionOverride != "" {
		service = &endpoints.ResolvedEndpoint{URL: fmt.Sprintf("https://%s", proxyURL.Host), SigningMethod: "v4", SigningRegion: p.RegionOverride, SigningName: p.SigningNameOverride}
	} else {
		service = determineAWSServiceFromHost(req.Host)
	}
	if service == nil {
		return nil, fmt.Errorf("unable to determine service from host: %s", req.Host)
	}

	if err := p.sign(proxyReq, service); err != nil {
		return nil, err
	}

	// When ContentLength is 0 we also need to set the body to http.NoBody to avoid Go http client
	// to magically set Transfer-Encoding: chunked. Service like S3 does not support chunk encoding.
	// We need to manipulate the Body value after signv4 signing because the signing process wraps
	// the original body into another struct, which will result in Transfer-Encoding: chunked being set.
	if proxyReq.ContentLength == 0 {
		proxyReq.Body = http.NoBody
	}

	// Remove any headers specified
	for _, header := range p.StripRequestHeaders {
		log.WithField("StripHeader", string(header)).Debug("Stripping Header:")
		req.Header.Del(header)
	}

	// Add origin headers after request is signed (no overwrite)
	copyHeaderWithoutOverwrite(proxyReq.Header, req.Header)

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
		b, _ := ioutil.ReadAll(resp.Body)
		log.WithField("request", fmt.Sprintf("%s %s", proxyReq.Method, proxyReq.URL)).
			WithField("status_code", resp.StatusCode).
			WithField("message", string(b)).
			Error("error proxying request")

		// Need to "reset" the response body because we consumed the stream above, otherwise caller will
		// get empty body.
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	}

	return resp, nil
}

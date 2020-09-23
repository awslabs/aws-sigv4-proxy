package handler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	log "github.com/sirupsen/logrus"
)

// Client is an interface to make testing http.Client calls easier
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// ProxyClient implements the Client interface
type ProxyClient struct {
	Signer *v4.Signer
	Client Client
	StripRequestHeaders []string
	SigningNameOverride string
	HostOverride string
	RegionOverride string
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

	if log.GetLevel() == log.DebugLevel && resp.StatusCode >= 400 {
		b, _ := ioutil.ReadAll(resp.Body)
		log.WithField("message", string(b)).Error("error proxying request")
	}

	return resp, nil
}

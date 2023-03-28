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

package main

import (
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	"aws-sigv4-proxy/handler"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug                  = kingpin.Flag("verbose", "Enable additional logging, implies all the log-* options").Short('v').Bool()
	logFailedResponse      = kingpin.Flag("log-failed-requests", "Log 4xx and 5xx response body").Bool()
	logSinging             = kingpin.Flag("log-signing-process", "Log sigv4 signing process").Bool()
	port                   = kingpin.Flag("port", "Port to serve http on").Default(":8080").String()
	strip                  = kingpin.Flag("strip", "Headers to strip from incoming request").Short('s').Strings()
	roleArn                = kingpin.Flag("role-arn", "Amazon Resource Name (ARN) of the role to assume").String()
	signingNameOverride    = kingpin.Flag("name", "AWS Service to sign for").String()
	signingHostOverride    = kingpin.Flag("sign-host", "Host to sign for").String()
	hostOverride           = kingpin.Flag("host", "Host to proxy to").String()
	regionOverride         = kingpin.Flag("region", "AWS region to sign for").String()
	disableSSLVerification = kingpin.Flag("no-verify-ssl", "Disable peer SSL certificate validation").Bool()
	idleConnTimeout        = kingpin.Flag("transport.idle-conn-timeout", "Idle timeout to the upstream service").Default("40s").Duration()
	schemeOverride         = kingpin.Flag("scheme", "Protocol to proxy with").String()
)

type awsLoggerAdapter struct {
}

// Log implements aws.Logger.Log
func (awsLoggerAdapter) Log(args ...interface{}) {
	log.Info(args...)
}

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	sessionConfig := aws.Config{}
	if v := os.Getenv("AWS_STS_REGIONAL_ENDPOINTS"); len(v) == 0 {
		sessionConfig.STSRegionalEndpoint = endpoints.RegionalSTSEndpoint
	}

	sessionConfig.CredentialsChainVerboseErrors = aws.Bool(shouldLogSigning())

	session, err := session.NewSession(&sessionConfig)
	if err != nil {
		log.Fatal(err)
	}

	if *regionOverride != "" {
		session.Config.Region = regionOverride
	}

	// For STS regional endpoint to be effective config's region must be set.
	if *session.Config.Region == "" {
		defaultRegion := "us-east-1"
		session.Config.Region = &defaultRegion
	}

	if *disableSSLVerification {
		log.Warn("Peer SSL Certificate validation is DISABLED")
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	http.DefaultTransport.(*http.Transport).IdleConnTimeout = *idleConnTimeout

	var credentials *credentials.Credentials
	if *roleArn != "" {
		credentials = stscreds.NewCredentials(session, *roleArn, func(p *stscreds.AssumeRoleProvider) {
			p.RoleSessionName = roleSessionName()
		})
	} else {
		credentials = session.Config.Credentials
	}

	signer := v4.NewSigner(credentials, func(s *v4.Signer) {
		if shouldLogSigning() {
			s.Logger = awsLoggerAdapter{}
			s.Debug = aws.LogDebugWithSigning
		}
	})
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	log.WithFields(log.Fields{"StripHeaders": *strip}).Infof("Stripping headers %s", *strip)
	log.WithFields(log.Fields{"port": *port}).Infof("Listening on %s", *port)

	log.Fatal(
		http.ListenAndServe(*port, &handler.Handler{
			ProxyClient: &handler.ProxyClient{
				Signer:              signer,
				Client:              client,
				StripRequestHeaders: *strip,
				SigningNameOverride: *signingNameOverride,
				SigningHostOverride: *signingHostOverride,
				HostOverride:        *hostOverride,
				RegionOverride:      *regionOverride,
				LogFailedRequest:    *logFailedResponse,
				SchemeOverride:      *schemeOverride,
			},
		}),
	)
}

func shouldLogSigning() bool {
	return *logSinging || *debug
}

func roleSessionName() string {
	suffix, err := os.Hostname()

	if err != nil {
		now := time.Now().Unix()
		suffix = strconv.FormatInt(now, 10)
	}

	return "aws-sigv4-proxy-" + suffix
}

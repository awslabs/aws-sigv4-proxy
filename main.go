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
    "net/http"
    "os"
    "strconv"
    "time"

    "aws-sigv4-proxy/handler"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/credentials/stscreds"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/signer/v4"
    log "github.com/sirupsen/logrus"
    "gopkg.in/alecthomas/kingpin.v2"
    "gopkg.in/yaml.v2"
)

var (
	debug = kingpin.Flag("verbose", "enable additional logging").Short('v').Bool()
	port  = kingpin.Flag("port", "port to serve http on").Default(":8080").String()
	strip = kingpin.Flag("strip", "Headers to strip from incoming request").Short('s').Strings()
	roleArn = kingpin.Flag("role-arn", "Amazon Resource Name (ARN) of the role to assume").String()
	signingNameOverride = kingpin.Flag("name", "AWS Service to sign for").String();
	hostOverride = kingpin.Flag("host", "Host to proxy to").String();
	regionOverride = kingpin.Flag("region", "AWS region to sign for").String();
	configSets = kingpin.Flag("config-set", "Host-based configuration overrides for role-arn/name/host/region (encoded as yaml)").Short('c').Strings()
)


func getSigner(roleArn *string) *v4.Signer {
    session, err := session.NewSession()
    if err != nil {
        log.Fatal(err)
    }

	var credentials *credentials.Credentials
	if *roleArn != "" {
	    credentials = stscreds.NewCredentials(session, *roleArn, func(p *stscreds.AssumeRoleProvider) {
		    p.RoleSessionName = roleSessionName()
	    })
	} else {
	    credentials = session.Config.Credentials
	}

	return v4.NewSigner(credentials)
}

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	signer := getSigner(roleArn)

	log.WithFields(log.Fields{"StripHeaders": *strip}).Infof("Stripping headers %s", *strip)
	log.WithFields(log.Fields{"port": *port}).Infof("Listening on %s", *port)

	clients := map[string]handler.Client{
		"default": &handler.ProxyClient{
			Signer: signer,
			Client: http.DefaultClient,
			StripRequestHeaders: *strip,
			SigningNameOverride: *signingNameOverride,
			HostOverride: *hostOverride,
			RegionOverride: *regionOverride,
		},
	}
	for _, configSetYaml := range *configSets {
		configSet := handler.ConfigSet{}

		if err := yaml.Unmarshal([]byte(configSetYaml), &configSet); err != nil {
			log.Fatalf("error parsing config set: %v", err)
		}

		log.WithFields(log.Fields{
			"Host": configSet.Host,
			"Name": configSet.Name,
			"Region": configSet.Region,
			"RoleArn": configSet.RoleArn,
		}).Info("Adding config for host")

		clients[configSet.Host] = &handler.ProxyClient{
			Signer: getSigner(&configSet.RoleArn),
			Client: http.DefaultClient,
			StripRequestHeaders: *strip,
			SigningNameOverride: configSet.Name,
			HostOverride: configSet.Host,
			RegionOverride: configSet.Region,
		}
	}

	log.Fatal(
		http.ListenAndServe(*port, &handler.Handler{
			ProxyClients: clients,
		}),
	)
}

func roleSessionName() string {
	suffix, err := os.Hostname()

	if err != nil {
		now := time.Now().Unix()
		suffix = strconv.FormatInt(now, 10)
	}

	return "aws-sigv4-proxy-" + suffix
}

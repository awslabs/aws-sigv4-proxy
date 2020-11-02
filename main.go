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
)

var (
	debug = kingpin.Flag("verbose", "enable additional logging").Short('v').Bool()
	port  = kingpin.Flag("port", "port to serve http on").Default(":8080").String()
	strip = kingpin.Flag("strip", "Headers to strip from incoming request").Short('s').Strings()
	roleArn = kingpin.Flag("role-arn", "Amazon Resource Name (ARN) of the role to assume").String()
	signingNameOverride = kingpin.Flag("name", "AWS Service to sign for").String();
	hostOverride = kingpin.Flag("host", "Host to proxy to").String();
	regionOverride = kingpin.Flag("region", "AWS region to sign for").String();
)

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

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

	signer := v4.NewSigner(credentials)

	log.WithFields(log.Fields{"StripHeaders": *strip}).Infof("Stripping headers %s", *strip)
	log.WithFields(log.Fields{"port": *port}).Infof("Listening on %s", *port)

	log.Fatal(
		http.ListenAndServe(*port, &handler.Handler{
			ProxyClient: &handler.ProxyClient{
				Signer: signer,
				Client: http.DefaultClient,
				StripRequestHeaders: *strip,
				SigningNameOverride: *signingNameOverride,
				HostOverride: *hostOverride,
				RegionOverride: *regionOverride,
			},
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

package main

import (
    "net/http"

    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/signer/v4"
    "github.com/awslabs/aws-sigv4-proxy/handler"
    log "github.com/sirupsen/logrus"
    "gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug = kingpin.Flag("verbose", "enable additional logging").Short('v').Bool()
	port  = kingpin.Flag("port", "port to serve http on").Default(":8080").String()
	strip = kingpin.Flag("strip", "Headers to strip from incoming request").Short('s').Strings()
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

	signer := v4.NewSigner(session.Config.Credentials)

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

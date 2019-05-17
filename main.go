package main

import (
	"net/http"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/awslabs/aws-sigv4-proxy/handler"
	"github.com/aws/aws-sdk-go/aws/defaults"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug = kingpin.Flag("verbose", "enable additional logging").Short('v').Bool()
	port  = kingpin.Flag("port", "port to serve http on").Default(":8080").String()
	strip = kingpin.Flag("strip", "Headers to strip from incoming request").Short('s').Strings()
)

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	// S3 signing is specialized and requires no escaping of the URI path,
	// so we have a default signer and an S3-specific signer.
	signer := v4.NewSigner(defaults.Get().Config.Credentials)
	s3Signer := v4.NewSigner(
		signer.Credentials,
		func(s *v4.Signer) {s.DisableURIPathEscaping = true},
	)

	log.WithFields(log.Fields{"StripHeaders": *strip}).Infof("Stripping headers %s", *strip)
	log.WithFields(log.Fields{"port": *port}).Infof("Listening on %s", *port)

	log.Fatal(
		http.ListenAndServe(*port, &handler.Handler{
			ProxyClient: &handler.ProxyClient{
				Signer: signer,
				S3Signer: s3Signer,
				Client: http.DefaultClient,
				StripRequestHeaders: *strip,
			},
		}),
	)
}

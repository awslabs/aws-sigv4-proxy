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
)

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

  cfg := defaults.Get().Config
  creds := cfg.Credentials

	signer := v4.NewSigner(creds)

	log.WithFields(log.Fields{"port": *port}).Infof("Listening on %s", *port)

	log.Fatal(
		http.ListenAndServe(*port, &handler.Handler{
			ProxyClient: &handler.ProxyClient{
				Signer: signer,
				Client: http.DefaultClient,
			},
		}),
	)
}

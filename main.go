package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	log "github.com/sirupsen/logrus"
	"github.com/tantona/aws-sigv4-proxy/handler"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug = kingpin.Flag("verbose", "enable additional logging").Short('v').Bool()
	port  = kingpin.Flag("port", "port to serve http on").Default(":8080").String()
)

func getCredentials() (*credentials.Credentials, error) {
	// Check env var, shared credentials, then finally ec2 instance role
	providers := []credentials.Provider{
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
		&ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(session.Must(session.NewSession())),
		},
	}

	creds := credentials.NewChainCredentials(providers)

	if _, err := creds.Get(); err != nil {
		return nil, err
	}

	return creds, nil
}

func main() {
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	creds, err := getCredentials()
	if err != nil {
		log.WithError(err).Fatal("unable to get credentials")
	}

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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/service"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "webhook-service"
	app.Version = VERSION
	app.Usage = "You need help!"
	app.Action = StartWebhook
	app.Commands = []cli.Command{}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "rsa-public-key-file",
			Usage: fmt.Sprintf(
				"Specify the path to the file containing RSA public key",
			),
		},
		cli.StringFlag{
			Name: "rsa-private-key-file",
			Usage: fmt.Sprintf(
				"Specify the path to the file containing RSA private key",
			),
		},
		cli.StringFlag{
			Name: "rsa-public-key-contents",
			Usage: fmt.Sprintf(
				"An alternative to  rsa-public-key-file. Specify the contents of the key.",
			),
			EnvVar: "RSA_PUBLIC_KEY_CONTENTS",
		},
		cli.StringFlag{
			Name: "rsa-private-key-contents",
			Usage: fmt.Sprintf(
				"An alternative to rsa-private-key-file. Specify the contents of the key.",
			),
			EnvVar: "RSA_PRIVATE_KEY_CONTENTS",
		},
		cli.StringFlag{
			Name: "kubeconfig",
			Usage: fmt.Sprintf(
				"Path to kubeconfig",
			),
			EnvVar: "KUBECONFIG",
		},
	}
	app.Run(os.Args)
}

func StartWebhook(c *cli.Context) {
	drivers.RegisterDrivers()
	privateKey, publicKey, err := service.GetKeys(c)
	if err != nil {
		log.Fatal("rsa-private-key-file or rsa-public-key-file not provided, halting")
	}

	rh := &service.RouteHandler{
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		ClientFactory: &service.ClientFactory{},
	}
	router := service.NewRouter(rh)

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()
	if kubeconfig != nil {
		if _, err := os.Stat(*kubeconfig); os.IsNotExist(err) {
			log.Infof("kubeconfig path provided but file not found")
			goto start
		}
		config.InitializeK8sClientSet(kubeconfig)
	}

start:
	log.Infof("Webhook service listening on 8085")
	log.Fatal(http.ListenAndServe(":8085", router))
}

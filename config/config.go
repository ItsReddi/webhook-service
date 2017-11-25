package config

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	CattleURL       string
	CattleAccessKey string
	CattleSecretKey string
}

func GetConfig() Config {
	config := Config{
		CattleURL:       os.Getenv("CATTLE_URL"),
		CattleAccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		CattleSecretKey: os.Getenv("CATTLE_SECRET_KEY"),
	}

	return config
}

var KubeClientSet *kubernetes.Clientset

func InitializeK8sClientSet(kubeconfig *string) *kubernetes.Clientset {
	log.Infof("Generating kubeconfig")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	KubeClientSet = clientset
	return clientset
}

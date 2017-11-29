package drivers

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	rConfig "github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"

	v1Get "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api/v1"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
)

type DeploymentUpdateDriver struct {
}

func (d *DeploymentUpdateDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.DeploymentUpdate)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.Name == "" {
		return http.StatusBadRequest, fmt.Errorf("Name not provided")
	}

	if config.Namespace == "" {
		return http.StatusBadRequest, fmt.Errorf("Namespace not provided")
	}

	return http.StatusOK, nil
}

func (d *DeploymentUpdateDriver) Execute(conf interface{}, apiClient *client.RancherClient, requestPayload interface{}) (int, error) {
	requestBody := make(map[string]interface{})
	config := &model.DeploymentUpdate{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	requestBody, ok := requestPayload.(map[string]interface{})
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Body should be of type map[string]interface{}")
	}

	pushedData, ok := requestBody["push_data"]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Incomplete webhook response provided")
	}

	pushedTag, ok := pushedData.(map[string]interface{})["tag"].(string)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Webhook response contains no tag")
	}

	repository, ok := requestBody["repository"]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Response provided without repository information")
	}

	imageName, ok := repository.(map[string]interface{})["repo_name"].(string)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Response provided without image name")
	}
	pushedImage := imageName + ":" + pushedTag
	log.Infof("[Webhook-service] Pushed image: %s", pushedImage)

	clientSet := rConfig.KubeClientSet
	currDepl, err := clientSet.AppsV1beta1().Deployments(config.Namespace).Get(config.Name, v1Get.GetOptions{})
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("Error in getting current deployment: %v", err)
	}

	newC := currDepl.Spec.Template.Spec.Containers[0]

	newC.Image = pushedImage
	newDepl := v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{newC},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(newDepl)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error in marshaling: %v", err)
	}

	log.Infof("[Webhook-service] Making patch request to update image")
	_, err = clientSet.AppsV1beta1().Deployments(config.Namespace).Patch(config.Name, types.MergePatchType, []byte(jsonBody))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error in updating deployment: %v", err)
	}

	log.Infof("[Webhook-service] Deployment %s updated", config.Name)
	return http.StatusOK, nil
}

func (d *DeploymentUpdateDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if upgradeConfig, ok := conf.(model.DeploymentUpdate); ok {
		webhook.DeploymentUpdateConfig = upgradeConfig
		webhook.DeploymentUpdateConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.DeploymentUpdate{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.DeploymentUpdateConfig = config
		webhook.DeploymentUpdateConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (d *DeploymentUpdateDriver) GetDriverConfigResource() interface{} {
	return model.DeploymentUpdate{}
}

func (d *DeploymentUpdateDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

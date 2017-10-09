package drivers

import (
	"fmt"
	"net/http"
	"net/url"
	// "regexp"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	// log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	rConfig "github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"
)

// var regTag = regexp.MustCompile(`^[\w]+[\w.-]*`)

type DeploymentUpdateDriver struct {
}

func (d *DeploymentUpdateDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.DeploymentUpdate)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.Tag == "" {
		return http.StatusBadRequest, fmt.Errorf("Tag not provided")
	}

	if config.Name == "" {
		return http.StatusBadRequest, fmt.Errorf("Name not provided")
	}

	if config.Namespace == "" {
		return http.StatusBadRequest, fmt.Errorf("Namespace not provided")
	}

	err := IsValidTag(config.Tag)
	if err != nil {
		return http.StatusBadRequest, err
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

	// requestedTag := config.Tag
	// if requestPayload == nil {
	// 	return http.StatusBadRequest, fmt.Errorf("No Payload recevied from webhook")
	// }

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
	fmt.Printf("pushedImage: %s\n", pushedImage)
	k8sURL := "/apis/apps/v1beta1/namespaces/" + config.Namespace + "/deployments/" + config.Name
	cattleConfig := rConfig.GetConfig()
	cattleURL := cattleConfig.CattleURL
	u, err := url.Parse(cattleURL)
	if err != nil {
		panic(err)
	}
	cattleURL = strings.Split(cattleURL, u.Path)[0] + "/r/projects/1a12/kubernetes:6443"
	k8sURL = cattleURL + k8sURL

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	request, err := http.NewRequest("GET", k8sURL, nil)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error creating request to get host: %v", err)
	}
	fmt.Printf("k8sURL: %s\n", k8sURL)

	// request.SetBasicAuth(cattleConfig.CattleAccessKey, cattleConfig.CattleSecretKey)
	resp, err := httpClient.Do(request)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	depl := make(map[string]interface{})

	err = json.Unmarshal(respBytes, &depl)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	fmt.Printf("respBytes: %v\n", depl)
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

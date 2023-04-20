package resources

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type ResourceMeta struct {
	Name      string
	Namespace string
}

type Resource struct {
	ResourceMeta
	GVK schema.GroupVersionKind
}

type ResourceConfiguration struct {
	GroupVersionKind schema.GroupVersionKind
	ControlledList   []ResourceMeta
}

type resourceFinderConfiguration struct {
	Resources []ResourceConfiguration
}

type IstioResourcesFinder struct {
	ctx           context.Context
	logger        logr.Logger
	client        client.Client
	configuration resourceFinderConfiguration
}

func NewIstioResourcesFinderFromConfigYaml(ctx context.Context, client client.Client, logger logr.Logger, path string) (*IstioResourcesFinder, error) {
	configYaml, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var finderConfiguration resourceFinderConfiguration
	err = yaml.Unmarshal(configYaml, &finderConfiguration)
	if err != nil {
		return nil, err
	}

	for _, resource := range finderConfiguration.Resources {
		for _, meta := range resource.ControlledList {
			_, err := regexp.Compile(meta.Name)
			if err != nil {
				return nil, fmt.Errorf("configuration yaml regex check failed for \"%s\": %s", meta.Name, err)
			}

			_, err = regexp.Compile(meta.Namespace)
			if err != nil {
				return nil, fmt.Errorf("configuration yaml regex check failed for \"%s\": %s", meta.Namespace, err)
			}
		}
	}

	return &IstioResourcesFinder{
		ctx:           ctx,
		logger:        logger,
		client:        client,
		configuration: finderConfiguration,
	}, nil
}

func (i *IstioResourcesFinder) FindUserCreatedIstioResources() ([]Resource, error) {
	var userResources []Resource
	for _, resource := range i.configuration.Resources {
		var u unstructured.UnstructuredList
		u.SetGroupVersionKind(resource.GroupVersionKind)
		err := i.client.List(i.ctx, &u)
		if err != nil {
			return nil, err
		}
		for _, item := range u.Items {
			res := Resource{
				GVK: resource.GroupVersionKind,
				ResourceMeta: ResourceMeta{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}

			managed, err := contains(resource.ControlledList, res.ResourceMeta)
			if err != nil {
				return nil, err
			}

			if !managed {
				userResources = append(userResources, res)
			}
		}
	}
	return userResources, nil
}
func contains(s []ResourceMeta, e ResourceMeta) (bool, error) {
	for _, r := range s {
		matchName, err := regexp.MatchString(r.Name, e.Name)
		if err != nil {
			return false, err
		}
		matchNamespace, err := regexp.MatchString(r.Namespace, e.Namespace)
		if err != nil {
			return false, err
		}
		if matchNamespace && matchName {
			return true, nil
		}
	}
	return false, nil
}

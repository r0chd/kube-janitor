package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Config struct {
	Directory string `yaml:"directory"`
}

type ResourceState struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type ManifestResource struct {
	Resource ResourceState
	Object   *unstructured.Unstructured
}

func main() {
	cfgBytes, err := os.ReadFile("/app/config.yaml")
	if err != nil {
		panic(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		panic(err)
	}

	fmt.Printf("Watching directory: %s\n", cfg.Directory)

	kcfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kcfg)
	if err != nil {
		panic(err)
	}

	dynamicClient, err := dynamic.NewForConfig(kcfg)
	if err != nil {
		panic(err)
	}

	ctx := context.TODO()

	for {
		liveResources, err := getLiveResources(ctx, discoveryClient, dynamicClient)
		if err != nil {
			log.Printf("Error getting live resources: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		manifestResources, err := parseManifests(cfg.Directory)
		if err != nil {
			log.Printf("Error parsing manifests: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		err = reconcile(ctx, dynamicClient, liveResources, manifestResources)
		if err != nil {
			log.Printf("Error during reconciliation: %v", err)
		}

		// TODO: watch directory for changes instead
		time.Sleep(10 * time.Second)
	}
}

func getLiveResources(ctx context.Context, discoveryClient *discovery.DiscoveryClient, dynamicClient dynamic.Interface) (map[ResourceState]*unstructured.Unstructured, error) {
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	liveResources := make(map[ResourceState]*unstructured.Unstructured)

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			if !slices.Contains(apiResource.Verbs, "list") {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiResource.Name,
			}

			var resourceInterface dynamic.ResourceInterface
			if apiResource.Namespaced {
				resourceInterface = dynamicClient.Resource(gvr).Namespace("")
			} else {
				resourceInterface = dynamicClient.Resource(gvr)
			}

			list, err := resourceInterface.List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Printf("Error listing %s: %v", gvr.String(), err)
				continue
			}

			for _, item := range list.Items {
				key := ResourceState{
					Group:     gv.Group,
					Version:   gv.Version,
					Kind:      item.GetKind(),
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				}
				liveResources[key] = item.DeepCopy()
			}
		}
	}

	return liveResources, nil
}

func parseManifests(directory string) (map[ResourceState]*unstructured.Unstructured, error) {
	manifestResources := make(map[ResourceState]*unstructured.Unstructured)

	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Error reading file %s: %v", path, err)
			return nil
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			obj := &unstructured.Unstructured{}
			if err := decoder.Decode(obj); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("Error decoding YAML in %s: %v", path, err)
				continue
			}

			if len(obj.Object) == 0 {
				continue
			}

			gvk := obj.GroupVersionKind()
			key := ResourceState{
				Group:     gvk.Group,
				Version:   gvk.Version,
				Kind:      gvk.Kind,
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}

			manifestResources[key] = obj
		}

		return nil
	})

	return manifestResources, err
}

func reconcile(ctx context.Context, dynamicClient dynamic.Interface, liveResources, manifestResources map[ResourceState]*unstructured.Unstructured) error {
	processed := make(map[ResourceState]bool)

	for key, manifestObj := range manifestResources {
		processed[key] = true

		gvr := schema.GroupVersionResource{
			Group:    key.Group,
			Version:  key.Version,
			Resource: getResourceNameFromKind(key.Kind),
		}

		var resourceInterface dynamic.ResourceInterface
		if key.Namespace != "" {
			resourceInterface = dynamicClient.Resource(gvr).Namespace(key.Namespace)
		} else {
			resourceInterface = dynamicClient.Resource(gvr)
		}

		liveObj, exists := liveResources[key]

		if !exists {
			// Create new resource
			fmt.Printf("Creating %s %s/%s\n", key.Kind, key.Namespace, key.Name)
			_, err := resourceInterface.Create(ctx, manifestObj, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating %s %s/%s: %v", key.Kind, key.Namespace, key.Name, err)
			}
		} else {
			// Update existing resource
			fmt.Printf("Updating %s %s/%s\n", key.Kind, key.Namespace, key.Name)
			manifestObj.SetResourceVersion(liveObj.GetResourceVersion())
			_, err := resourceInterface.Update(ctx, manifestObj, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Error updating %s %s/%s: %v", key.Kind, key.Namespace, key.Name, err)
			}
		}
	}

	// Delete resources not in manifests (prune)
	for key := range liveResources {
		if !processed[key] {
			gvr := schema.GroupVersionResource{
				Group:    key.Group,
				Version:  key.Version,
				Resource: getResourceNameFromKind(key.Kind),
			}

			var resourceInterface dynamic.ResourceInterface
			if key.Namespace != "" {
				resourceInterface = dynamicClient.Resource(gvr).Namespace(key.Namespace)
			} else {
				resourceInterface = dynamicClient.Resource(gvr)
			}

			fmt.Printf("Deleting %s %s/%s\n", key.Kind, key.Namespace, key.Name)
			err := resourceInterface.Delete(ctx, key.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Error deleting %s %s/%s: %v", key.Kind, key.Namespace, key.Name, err)
			}
		}
	}

	return nil
}

func getResourceNameFromKind(kind string) string {
	kindToResource := map[string]string{
		"Pod":         "pods",
		"Deployment":  "deployments",
		"ReplicaSet":  "replicasets",
		"Service":     "services",
		"ConfigMap":   "configmaps",
		"Secret":      "secrets",
		"DaemonSet":   "daemonsets",
		"StatefulSet": "statefulsets",
		"Ingress":     "ingresses",
		"Job":         "jobs",
		"CronJob":     "cronjobs",
	}

	if resource, exists := kindToResource[kind]; exists {
		return resource
	}

	lower := strings.ToLower(kind)
	if !strings.HasSuffix(lower, "s") {
		return lower + "s"
	}
	return lower
}

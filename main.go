package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

func main() {
	conf := flag.String("config", "", "path to config")
	fmt.Printf("%s", conf)

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	f, err := os.ReadFile("/app/config.yaml")
	if err != nil {
		panic(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		panic(err)
	}

	fmt.Printf("Host: %s, Port: %d\n", cfg.Host, cfg.Port)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	for {
		_, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		//for _, _ := range pods.Items {
		//}
	}
}

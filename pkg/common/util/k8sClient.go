package util

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	logr "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var parsed bool = false
var kubeconfig *string

func isInCluster() bool {
	if _, ok := os.LookupEnv("KUBERNETES_PORT"); ok {
		return true
	}

	return false
}

func ConnectK8sClient() *kubernetes.Clientset {
	if isInCluster() {
		return ConnectInClusterAPIClient()
	}

	return ConnectLocalAPIClient()
}

func ConnectLocalAPIClient() *kubernetes.Clientset {
	if !parsed {
		homeDir := ""
		if h := os.Getenv("HOME"); h != "" {
			homeDir = h
		} else {
			homeDir = os.Getenv("USERPROFILE") // windows
		}

		envKubeConfig := os.Getenv("KUBECONFIG")
		if envKubeConfig != "" {
			kubeconfig = &envKubeConfig
		} else {
			if home := homeDir; home != "" {
				kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
		}

		parsed = true
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		logr.WithError(err).Error("Failed to create config")
		return nil
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logr.WithError(err).Error("Failed to create clientset")
		return nil
	}

	return clientset
}

func ConnectInClusterAPIClient() *kubernetes.Clientset {
	host := ""
	port := ""
	token := ""

	if val, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
		host = val
	} else {
		host = "127.0.0.1"
	}

	if val, ok := os.LookupEnv("KUBERNETES_PORT_443_TCP_PORT"); ok {
		port = val
	} else {
		port = "6443"
	}

	read, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		logr.WithError(err).Error("Failed to read token")
		return nil
	}

	token = string(read)

	// create the configuration by token
	kubeConfig := &rest.Config{
		Host:        "https://" + host + ":" + port,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	if client, err := kubernetes.NewForConfig(kubeConfig); err != nil {
		logr.WithError(err).Error("Failed to create client")
		return nil
	} else {
		return client
	}
}

func CreateK8sSecrets(namespace, secretname string, data map[string][]byte) error {

	client := ConnectK8sClient()

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretname,
		},
		Data: data,
		Type: v1.SecretTypeOpaque,
	}

	oldSec, err := GetK8sSecrets(namespace, secretname)
	if err == nil {

		oldData := oldSec.Data

		for k, v := range oldData {
			data[k] = v
		}
		oldSec.Data = data
		_, err := client.CoreV1().Secrets(namespace).Update(context.Background(), &oldSec, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = client.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})

		if err != nil {
			return err
		}
	}

	return nil
}

func GetK8sSecrets(namespace, secretname string) (v1.Secret, error) {
	client := ConnectK8sClient()
	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), secretname, metav1.GetOptions{})
	return *secret, err
}

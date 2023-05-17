package util

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var kubeconfig string

func isInCluster() bool {
	if _, ok := os.LookupEnv("KUBERNETES_PORT"); ok {
		return true
	}
	return false
}

func ConnectK8sClient() *kubernetes.Clientset {
	if kubeconfig == "" {
		kubeconfig = setDefaultKubePath()
	}
	if isInCluster() {
		return ConnectInClusterAPIClient()
	}

	return ConnectLocalAPIClient(kubeconfig)
}

func setDefaultKubePath() string {

	homeDir := ""
	if h := os.Getenv("HOME"); h != "" {
		homeDir = h
	} else {
		homeDir = os.Getenv("USERPROFILE") // windows
	}

	envKubeConfig := os.Getenv("KUBECONFIG")
	if envKubeConfig != "" {
		kubeconfig = envKubeConfig
	} else {
		if home := homeDir; home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	return kubeconfig
}

func ConnectLocalAPIClient(kubeConfig string) *kubernetes.Clientset {

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		log.WithError(err).Error("Failed to create config")
		return nil
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Error("Failed to create clientset")
		return nil
	}

	return clientset
}

func ConnectInClusterAPIClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.WithError(err).Error("Failed to create clientset")
		return nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil
	}
	return clientset
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
		log.WithField("secret", oldSec.Name).Info("Found k8s secret with same name. Trying to update existing secret")
		if oldSec.Data == nil {
			oldSec.Data = map[string][]byte{}
		}
		for k, value := range data {
			oldSec.Data[k] = value
		}
		_, err := client.CoreV1().Secrets(namespace).Update(context.Background(), &oldSec, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		log.Info("No k8s secret found. Trying to create new secret")
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

func deleteSecret(namespace, secretname string) error {
	client := ConnectK8sClient()
	return client.CoreV1().Secrets(namespace).Delete(context.Background(), secretname, metav1.DeleteOptions{})
}

func DeleteK8sSecrets(namespace, secretname, typeString string) error {
	secret, err := GetK8sSecrets(namespace, secretname)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	log.WithField("secret", secret.Name).Info("Secret found. Trying to delete now")
	mapData := make(map[string][]byte)
	for key, value := range secret.Data {
		if !strings.Contains(key, typeString) {
			mapData[key] = value
		}
	}
	err = deleteSecret(namespace, secretname)
	if err != nil {
		return err
	}
	log.WithField("secret=%v", secret.Name).Info("Successfully deleted secret")
	return CreateK8sSecrets(namespace, secretname, mapData)
}

package util

import (
	"context"
	"errors"
	"flag"
	"io"
	"maps"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	SecretKeyAgentData = "agent-data"
	SecretKeyKeys      = "keys"
	SecretKeyKeysTime  = "keys-time"
)

var (
	parsed           bool = false
	kubeConfig       *string
	ErrNoSecretFound = errors.New("no secrets found")
	ErrNoPKIFound    = errors.New("no private key found")
)

func isInCluster() bool {
	if _, ok := os.LookupEnv("KUBERNETES_PORT"); ok {
		return true
	}
	return false
}

func ConnectK8sClient() (*kubernetes.Clientset, error) {
	if isInCluster() {
		return connectInClusterAPIClient()
	}
	return connectLocalAPIClient()
}

func connectLocalAPIClient() (*kubernetes.Clientset, error) {
	if !parsed {
		homeDir := ""
		if h := os.Getenv("HOME"); h != "" {
			homeDir = h
		} else {
			homeDir = os.Getenv("USERPROFILE") // windows
		}

		envKubeConfig := os.Getenv("KUBECONFIG")
		if envKubeConfig != "" {
			kubeConfig = &envKubeConfig
		} else {
			if home := homeDir; home != "" {
				kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
		}

		parsed = true
	}
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		return nil, err
	}
	// return the clientset
	return kubernetes.NewForConfig(config)
}

func connectInClusterAPIClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func shouldRetry(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

func backoff(attempt int) time.Duration {
	base := time.Second
	max := 30 * time.Second
	d := min(time.Duration(1<<attempt)*base, max)
	jitter := time.Duration(rand.Int63n(int64(d / 3)))
	return d - jitter
}

func LoadDataWithBackoff(namespace, secretName string) ([]byte, error) {
	if namespace == "" || secretName == "" {
		return nil, nil
	}
	client, err := ConnectK8sClient()
	if err != nil {
		return nil, err
	}

	var secretData map[string][]byte
	for i := range 3 {
		secret, err := getSecret(client, namespace, secretName)
		if err == nil || errors.Is(err, ErrNoSecretFound) || errors.Is(err, ErrNoPKIFound) {
			if secret.Data != nil {
				secretData = secret.Data
			}
			break
		}
		if !shouldRetry(err) {
			return nil, err
		}
		time.Sleep(backoff(i))
	}
	marshaled, ok := secretData[SecretKeyAgentData]
	if !ok || len(marshaled) == 0 {
		return nil, nil
	}
	return marshaled, nil
}

func getSecret(client *kubernetes.Clientset,
	namespace, secretName string) (*v1.Secret, error) {
	return client.
		CoreV1().
		Secrets(namespace).
		Get(context.Background(), secretName, metav1.GetOptions{})
}

func StoreDataWithBackoff(namespace, secretName string, marshaled []byte) error {
	if namespace == "" || secretName == "" {
		return nil
	}
	client, err := ConnectK8sClient()
	if err != nil {
		return err
	}

	secret, oldSecretFound := makeSecret(client, namespace, secretName)
	secret.Data[SecretKeyAgentData] = marshaled

	for i := range 3 {
		err = writeSecret(client, secret, oldSecretFound)
		if err == nil {
			return nil
		}
		if !shouldRetry(err) {
			return err
		}
		time.Sleep(backoff(i))
	}
	return nil
}

func writeSecret(client *kubernetes.Clientset,
	secret *v1.Secret,
	oldSecretFound bool) error {
	if oldSecretFound {
		_, err := client.
			CoreV1().
			Secrets(secret.Namespace).
			Update(context.Background(), secret, metav1.UpdateOptions{})
		return err
	}
	_, err := client.
		CoreV1().
		Secrets(secret.Namespace).
		Create(context.Background(), secret, metav1.CreateOptions{})
	return err
}

func LoadEntriesWithBackoff(namespace, secret string, retries int) ([]byte, error) {

	var (
		err       error
		dataBytes []byte
	)

	client, err := ConnectK8sClient()
	if err != nil {
		return nil, err
	}

	for i := range retries {
		dataBytes, err = getDataBytes(client, namespace, secret)
		if err == nil || errors.Is(err, ErrNoSecretFound) || errors.Is(err, ErrNoPKIFound) {
			break
		}
		if !shouldRetry(err) {
			return nil, err
		}
		time.Sleep(backoff(i))
	}

	if dataBytes == nil && err == nil {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return dataBytes, nil
}

func getDataBytes(client *kubernetes.Clientset, namespace, secretName string) ([]byte, error) {

	secret, err := getSecret(client, namespace, secretName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrNoSecretFound
		}
		return nil, err
	}

	dataBytes, ok := secret.Data[SecretKeyKeys]
	if !ok {
		return nil, ErrNoPKIFound
	}
	return dataBytes, nil
}

func WriteEntriesWithBackoff(namespace, secretName string, retries int, jsonBytes []byte) error {

	if namespace == "" || secretName == "" {
		return nil
	}
	client, err := ConnectK8sClient()
	if err != nil {
		return err
	}

	timeBytes, timeErr := time.Now().MarshalText()
	if timeErr != nil {
		return timeErr
	}

	secret, oldSecretFound := makeSecret(client, namespace, secretName)
	secret.Data[SecretKeyKeys] = jsonBytes
	secret.Data[SecretKeyKeysTime] = timeBytes

	for i := range retries {
		err = writeSecret(client, secret, oldSecretFound)
		if err == nil {
			return nil
		}
		if !shouldRetry(err) {
			return err
		}
		time.Sleep(backoff(i))
	}
	return err
}

func makeSecret(client *kubernetes.Clientset,
	namespace, secretName string) (*v1.Secret, bool) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
		Type: v1.SecretTypeOpaque,
	}
	oldSecretFound := false
	oldSecret, err := getSecret(client, namespace, secretName)
	if err == nil {
		if oldSecret.Data == nil {
			oldSecret.Data = make(map[string][]byte)
		}
		oldSecretFound = true
		maps.Copy(oldSecret.Data, secret.Data)
		secret = oldSecret
	}

	return secret, oldSecretFound
}

package sqlstore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	vapi "github.com/hashicorp/vault/api"
	vk8s "github.com/hashicorp/vault/api/auth/kubernetes"
)

type vaultConfig struct {
	Enabled     bool     `hcl:"enabled"`
	Server      string   `hcl:"server"`
	SkipVerify  bool     `hcl:"skip_tls_verify"`
	K8sAuthPath string   `hcl:"k8s_auth_path"`
	Namespace   string   `hcl:"namespace"`
	Role        string   `hcl:"role"`
	TokenPath   string   `hcl:"token_path"`
	Secret      *secrets `hcl:"secrets"`
}

type secrets struct {
	Path     string `hcl:"path"`
	Username string `hcl:"username"`
	Password string `hcl:"password"`
}

func (p *Plugin) initializeVault(cfg *vaultConfig) error {

	vaultCfg := vapi.DefaultConfig()
	vaultCfg.Address = cfg.Server
	vaultCfg.HttpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipVerify,
		},
	}
	client, err := vapi.NewClient(vaultCfg)
	if err != nil {
		return err
	}

	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	if cfg.Role == "" {
		cfg.Role = getRoleFromServiceAccount()
	}

	err = authWithServiceAccountToken(client, cfg.K8sAuthPath, cfg.Role, cfg.TokenPath)
	if err != nil {
		return err
	}

	p.vaultClient = client
	return nil
}

func getUsernamePasswordFromVault(cfg *vaultConfig, client *vapi.Client) (string, string, error) {

	client.SetClientTimeout(time.Minute)

	data, err := client.Logical().Read(cfg.Secret.Path)
	if err != nil {
		return "", "", err
	}

	var username, password string

	dataMap := normalizeSecretData(data)

	if val, ok := dataMap[cfg.Secret.Username].(string); ok {
		username = val
	}
	if val, ok := dataMap[cfg.Secret.Password].(string); ok {
		password = val
	}

	return username, password, nil
}

func authWithServiceAccountToken(client *vapi.Client, authPath, role, tokenPath string) error {
	var (
		opt     []vk8s.LoginOption
		k8sAuth *vk8s.KubernetesAuth
		err     error
	)

	if authPath != "" {
		opt = append(opt, vk8s.WithMountPath(authPath))
		if tokenPath != "" {
			opt = append(opt, vk8s.WithServiceAccountTokenPath(tokenPath))
		}
		k8sAuth, err = vk8s.NewKubernetesAuth(role, opt...)
	} else {
		k8sAuth, err = vk8s.NewKubernetesAuth(role)
	}
	if err != nil {
		return err
	}

	authInfo, err := client.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {

		return err
	}
	if authInfo == nil {
		return errors.New("no auth info was returned after login")
	}
	return err
}

func getRoleFromServiceAccount() string {

	var tokeninfo struct {
		KubernetesIO struct {
			Namespace string `json:"namespace"`

			ServiceAccount struct {
				Name string `json:"name"`
			} `json:"serviceaccount"`
		} `json:"kubernetes.io"`
	}

	saToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return ""
	}
	// Parse and validate JWT token using jwt v5
	parsedToken, _, err := jwt.NewParser().ParseUnverified(string(saToken), jwt.MapClaims{})
	if err != nil {
		return ""
	}

	// Convert claims to JSON
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	// Marshal claims into JSON
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return ""
	}

	// Unmarshal JSON into the token struct
	err = json.Unmarshal(claimsJSON, &tokeninfo)
	if err != nil {
		return ""
	}

	return tokeninfo.KubernetesIO.Namespace + "_" + tokeninfo.KubernetesIO.ServiceAccount.Name
}

func normalizeSecretData(data *vapi.Secret) map[string]any {
	if data == nil {
		return nil
	}
	if nested, ok := data.Data["data"].(map[string]any); ok {
		return nested
	}
	return data.Data
}

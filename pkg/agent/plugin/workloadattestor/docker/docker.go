package docker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	workloadattestorv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/accuknox/spire/pkg/common/catalog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl"
	"github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	pluginName                   = "docker"
	subselectorLabel             = "label"
	subselectorImageID           = "image_id"
	subselectorEnv               = "env"
	subselectorImageConfigDigest = "image_config_digest"
)

func BuiltIn() catalog.BuiltIn {
	return builtin(New())
}

func builtin(p *Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		workloadattestorv1.WorkloadAttestorPluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

// Docker is a subset of the docker client functionality, useful for mocking.
type Docker interface {
	ContainerInspect(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error)
	ImageInspect(ctx context.Context, imageID string, inspectOpts ...dockerclient.ImageInspectOption) (dockerclient.ImageInspectResult, error)
}

type Plugin struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer
	configv1.UnsafeConfigServer

	log     hclog.Logger
	retryer *retryer

	mtx    sync.RWMutex
	docker Docker
	c      *containerHelper
}

func New() *Plugin {
	return &Plugin{
		retryer: newRetryer(),
	}
}

type dockerPluginConfig struct {
	OSConfig `hcl:",squash"`

	// DockerVersion is the API version of the docker daemon. If not specified, the version is negotiated by the client.
	DockerVersion string `hcl:"docker_version" json:"docker_version"`

	UnusedKeys []string `hcl:",unusedKeys"`
}

func (p *Plugin) SetLogger(log hclog.Logger) {
	p.log = log
}

func (p *Plugin) Attest(ctx context.Context, req *workloadattestorv1.AttestRequest) (*workloadattestorv1.AttestResponse, error) {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	containerID, err := p.c.getContainerID(req.Pid, p.log)
	switch {
	case err != nil:
		return nil, err
	case containerID == "":
		// Not a docker workload. Nothing more to do.
		return &workloadattestorv1.AttestResponse{}, nil
	}

	var containerResult dockerclient.ContainerInspectResult
	err = p.retryer.Retry(ctx, func() error {
		containerResult, err = p.docker.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	selectors := getSelectorValuesFromConfig(containerResult.Container.Config)

	var imageJSON dockerclient.ImageInspectResult
	var inspectErr error
	imageName := containerResult.Container.Config.Image
	if imageName != "" {
		imageJSON, inspectErr = p.docker.ImageInspect(ctx, imageName)
	}

	// Add image_config_digest selector
	if inspectErr == nil && imageJSON.ID != "" {
		selectors = append(selectors, fmt.Sprintf("%s:%s", subselectorImageConfigDigest, imageJSON.ID))
	}

	return &workloadattestorv1.AttestResponse{
		SelectorValues: selectors,
	}, nil
}

func getSelectorValuesFromConfig(cfg *container.Config) []string {
	var selectorValues []string
	for label, value := range cfg.Labels {
		selectorValues = append(selectorValues, fmt.Sprintf("%s:%s:%s", subselectorLabel, label, value))
	}
	for _, e := range cfg.Env {
		selectorValues = append(selectorValues, fmt.Sprintf("%s:%s", subselectorEnv, e))
	}
	if cfg.Image != "" {
		selectorValues = append(selectorValues, fmt.Sprintf("%s:%s", subselectorImageID, cfg.Image))
	}
	return selectorValues
}

func (p *Plugin) Configure(ctx context.Context, req *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	var err error
	config := &dockerPluginConfig{}
	if err = hcl.Decode(config, req.HclConfiguration); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unable to decode configuration: %v", err)
	}

	if len(config.UnusedKeys) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "unknown configurations detected: %s", strings.Join(config.UnusedKeys, ","))
	}

	containerHelper, err := createHelper(config)
	if err != nil {
		return nil, err
	}

	var opts []dockerclient.Opt
	dockerHost := getDockerHost(config)
	if dockerHost != "" {
		opts = append(opts, dockerclient.WithHost(dockerHost))
	}

	if config.DockerVersion != "" {
		opts = append(opts, dockerclient.WithAPIVersion(config.DockerVersion))
	}

	docker, err := dockerclient.New(opts...)
	if err != nil {
		return nil, err
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.docker = docker
	p.c = containerHelper
	return &configv1.ConfigureResponse{}, nil
}

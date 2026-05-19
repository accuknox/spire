package k8ssat

import (
	"context"
	"fmt"
	"sync"
	"time"

	workloadattestorv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/pkg/common/pluginconf"
	"github.com/accuknox/spire/pkg/common/telemetry"
	"github.com/accuknox/spire/pkg/common/util"
	"github.com/andres-erbsen/clock"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl"
	hcltoken "github.com/hashicorp/hcl/hcl/token"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
)

const (
	pluginName               = "k8s_sat"
	defaultMaxPollAttempts   = 60
	defaultPollRetryInterval = time.Second * 5
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

// HCLConfig holds the configuration parsed from HCL
type HCLConfig struct {
	MaxPollAttempts   int    `hcl:"max_poll_attempts"`
	PollRetryInterval string `hcl:"poll_retry_interval"`
	ReloadInterval    string `hcl:"reload_interval"`

	UnusedKeyPositions map[string][]hcltoken.Pos `hcl:",unusedKeyPositions"`
}

type k8sSatConfig struct {
	MaxPollAttempts   int
	PollRetryInterval time.Duration
}

func (p *Plugin) buildConfig(coreConfig catalog.CoreConfig, hclText string, status *pluginconf.Status) *k8sSatConfig {
	// Parse HCL config payload into config struct
	newConfig := new(HCLConfig)
	if err := hcl.Decode(newConfig, hclText); err != nil {
		status.ReportErrorf("unable to decode configuration: %v", err)
		return nil
	}

	pluginconf.ReportUnusedKeys(status, newConfig.UnusedKeyPositions)

	// Determine max poll attempts with default
	maxPollAttempts := newConfig.MaxPollAttempts
	if maxPollAttempts <= 0 {
		maxPollAttempts = defaultMaxPollAttempts
	}

	// Determine poll retry interval with default
	var pollRetryInterval time.Duration
	var err error
	if newConfig.PollRetryInterval != "" {
		pollRetryInterval, err = time.ParseDuration(newConfig.PollRetryInterval)
		if err != nil {
			status.ReportErrorf("unable to parse poll retry interval: %v", err)
		}
	}
	if pollRetryInterval <= 0 {
		pollRetryInterval = defaultPollRetryInterval
	}

	return &k8sSatConfig{
		MaxPollAttempts:   maxPollAttempts,
		PollRetryInterval: pollRetryInterval,
	}
}

type Plugin struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer
	configv1.UnsafeConfigServer

	log    hclog.Logger
	clock  clock.Clock
	client *kubernetes.Clientset

	config *k8sSatConfig
	mu     sync.RWMutex
}

func New() *Plugin {
	return &Plugin{
		clock: clock.New(),
	}
}

func (p *Plugin) SetLogger(log hclog.Logger) {
	p.log = log
}

func (p *Plugin) Attest(ctx context.Context, req *workloadattestorv1.AttestRequest) (*workloadattestorv1.AttestResponse, error) {

	config := p.config
	if config == nil {
		return &workloadattestorv1.AttestResponse{}, status.Error(codes.FailedPrecondition, "not configured")
	}

	var err error
	if p.client == nil {
		p.client, err = util.ConnectK8sClient()
		if err != nil {
			return &workloadattestorv1.AttestResponse{}, fmt.Errorf("failed to create k8s client: %w", err)
		}
	}

	workload := req.Metadata["workload"]
	check := req.Metadata["check"]

	log := p.log

	for attempt := 1; ; attempt++ {
		log = log.With(telemetry.Attempt, attempt)

		var (
			selectorValues []string
			attestResponse *workloadattestorv1.AttestResponse
		)

		log.Info("Getting selector values", "workload", workload)

		if check == "health" {
			log.Info("Found health check request", "workload", workload)
			return nil, nil
		}

		token, ok := req.Metadata["sa_token"]
		if !ok {
			return nil, fmt.Errorf("no sa_token found in metadata for workload: %v", workload)
		}

		selectors, err := p.validateServiceAccountToken(token, log)
		if err != nil {
			return &workloadattestorv1.AttestResponse{}, err
		}

		selectorValues = append(selectorValues, selectors...)

		if len(selectorValues) > 0 {
			attestResponse = &workloadattestorv1.AttestResponse{SelectorValues: selectorValues}
		}

		if attestResponse != nil {
			return attestResponse, nil
		}

		// if the container was not located after the maximum number of attempts then the search is over.

		if attempt >= config.MaxPollAttempts {
			log.Warn("Decoding failed, giving up")
			return nil, status.Error(codes.DeadlineExceeded, "no selectors found after max poll attempts")
		}

		// wait a bit for containers to initialize before trying again.
		log.Debug("pod selectors not found, retrying", telemetry.RetryInterval, config.PollRetryInterval)

		select {
		case <-p.clock.After(config.PollRetryInterval):
		case <-ctx.Done():
			return nil, status.Errorf(codes.Canceled, "no selectors found: %v", ctx.Err())
		}
	}
}

func (p *Plugin) Configure(ctx context.Context, req *configv1.ConfigureRequest) (resp *configv1.ConfigureResponse, err error) {

	newConfig, _, err := pluginconf.Build(req, p.buildConfig)
	if err != nil {
		return nil, err
	}

	p.client, err = util.ConnectK8sClient()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = newConfig

	return &configv1.ConfigureResponse{}, nil
}

func (p *Plugin) Validate(_ context.Context, req *configv1.ValidateRequest) (resp *configv1.ValidateResponse, err error) {
	_, notes, err := pluginconf.Build(req, p.buildConfig)

	return &configv1.ValidateResponse{
		Valid: err == nil,
		Notes: notes,
	}, nil
}

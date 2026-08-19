package monitoring

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	spirelog "github.com/spiffe/spire/pkg/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIsDisabledWithoutDSN(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")

	monitor, err := New("spire-server", "test-release")

	require.NoError(t, err)
	assert.False(t, monitor.Enabled())
}

func TestNewUsesServiceSpecificFallbackRelease(t *testing.T) {
	t.Setenv("SENTRY_DSN", "https://public@example.com/1")
	t.Setenv("SENTRY_RELEASE", "")

	monitor, err := New("spire-server", "v1.2.3")

	require.NoError(t, err)
	require.True(t, monitor.Enabled())
	assert.Equal(t, "spire-server@v1.2.3", monitor.hub.Client().Options().Release)
}

func TestLogHookCapturesErrorsAndScrubsSecrets(t *testing.T) {
	transport := newRecordingTransport()
	monitor := newTestMonitor(t, "spire-server", transport)
	logger, err := spirelog.NewLogger(
		spirelog.WithOutputWriter(testWriter{t}),
		monitor.LogOption(),
	)
	require.NoError(t, err)

	logger.WithFields(logrus.Fields{
		"token":    "token-value",
		"database": "password=database-password host=postgres",
	}).WithError(errors.New("connection failed password=error-password")).Error("Server crashed")

	event := requireSingleEvent(t, transport)
	assert.Equal(t, "spire-server", event.Tags["service"])
	assert.Equal(t, sentry.LevelError, event.Level)
	require.Len(t, event.Exception, 1)
	assert.NotContains(t, event.Exception[0].Value, "error-password")
	assert.Equal(t, filteredValue, event.Contexts["log"]["token"])
	assert.NotContains(t, event.Contexts["log"]["database"], "database-password")
}

func TestLogHookCapturesOnlyHealthTransitions(t *testing.T) {
	transport := newRecordingTransport()
	monitor := newTestMonitor(t, "spire-agent", transport)
	logger, err := spirelog.NewLogger(
		spirelog.WithOutputWriter(testWriter{t}),
		monitor.LogOption(),
	)
	require.NoError(t, err)

	logger.WithField("check", "catalog.datastore").Error(repeatedHealthFailureLog)
	logger.Warn("ordinary warning")
	logger.WithField("check", "catalog.datastore").Warn(healthFailureTransitionLog)

	event := requireSingleEvent(t, transport)
	assert.Equal(t, sentry.LevelWarning, event.Level)
	assert.Equal(t, "catalog.datastore", event.Tags["health_check"])
	assert.Equal(t, []string{"spire-health", "catalog.datastore"}, event.Fingerprint)
}

func TestRecoverCapturesAndRepanics(t *testing.T) {
	transport := newRecordingTransport()
	monitor := newTestMonitor(t, "spire-server", transport)

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		defer monitor.Recover()
		panic("unexpected panic")
	}()

	assert.Equal(t, "unexpected panic", recovered)
	assert.True(t, monitor.Flush())
	event := requireSingleEvent(t, transport)
	assert.Equal(t, sentry.LevelFatal, event.Level)
	assert.Equal(t, "unexpected panic", event.Message)
}

func newTestMonitor(t *testing.T, serviceName string, transport sentry.Transport) *Monitor {
	t.Helper()
	monitor, err := newMonitor(serviceName, sentry.ClientOptions{
		Dsn:              "https://public@example.com/1",
		Transport:        transport,
		AttachStacktrace: true,
		BeforeSend:       scrubEvent,
	})
	require.NoError(t, err)
	return monitor
}

func requireSingleEvent(t *testing.T, transport *recordingTransport) *sentry.Event {
	t.Helper()
	transport.mu.Lock()
	defer transport.mu.Unlock()
	require.Len(t, transport.events, 1)
	return transport.events[0]
}

type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func newRecordingTransport() *recordingTransport {
	return &recordingTransport{}
}

func (*recordingTransport) Configure(sentry.ClientOptions) {}

func (t *recordingTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (*recordingTransport) Flush(time.Duration) bool { return true }

func (*recordingTransport) FlushWithContext(context.Context) bool { return true }

func (*recordingTransport) Close() {}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}

package monitoring

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	spirelog "github.com/accuknox/spire/pkg/common/log"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

const (
	filteredValue              = "[Filtered]"
	defaultFlushTimeout        = 2 * time.Second
	repeatedHealthFailureLog   = "Health check has failed"
	healthFailureTransitionLog = "Health check failed"
)

var sensitiveValuePattern = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|authorization|api[_-]?key|dsn)\s*([=:])\s*("[^"]*"|'[^']*'|[^\s,;]+)`)

// Monitor connects SPIRE's existing Logrus logger to Sentry. A monitor with
// no hub is disabled and all of its operations are safe no-ops.
type Monitor struct {
	hub  *sentry.Hub
	hook *sentryHook
}

// New creates a monitor from the standard Sentry environment variables. An
// empty SENTRY_DSN disables monitoring without affecting SPIRE startup.
func New(serviceName, fallbackRelease string) (*Monitor, error) {
	dsn := strings.TrimSpace(os.Getenv("SENTRY_DSN"))
	if dsn == "" {
		return Disabled(), nil
	}

	release := strings.TrimSpace(os.Getenv("SENTRY_RELEASE"))
	if release == "" {
		release = serviceName + "@" + fallbackRelease
	}

	return newMonitor(serviceName, sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      strings.TrimSpace(os.Getenv("SENTRY_ENVIRONMENT")),
		Release:          release,
		AttachStacktrace: true,
		EnableTracing:    false,
		SendDefaultPII:   false,
		BeforeSend:       scrubEvent,
	})
}

func newMonitor(serviceName string, options sentry.ClientOptions) (*Monitor, error) {
	client, err := sentry.NewClient(options)
	if err != nil {
		return nil, fmt.Errorf("initialize Sentry client: %w", err)
	}

	scope := sentry.NewScope()
	scope.SetTag("service", serviceName)
	hub := sentry.NewHub(client, scope)

	return &Monitor{
		hub:  hub,
		hook: &sentryHook{hub: hub},
	}, nil
}

// Disabled returns a monitor that does not submit events.
func Disabled() *Monitor {
	return &Monitor{}
}

// Enabled reports whether the monitor has an active Sentry client.
func (m *Monitor) Enabled() bool {
	return m != nil && m.hub != nil
}

// LogOption attaches Sentry monitoring to a SPIRE logger.
func (m *Monitor) LogOption() spirelog.Option {
	return func(logger *spirelog.Logger) error {
		if m != nil && m.hook != nil {
			logger.AddHook(m.hook)
		}
		return nil
	}
}

// Flush waits briefly for queued events to be delivered.
func (m *Monitor) Flush() bool {
	if !m.Enabled() {
		return true
	}
	return m.hub.Flush(defaultFlushTimeout)
}

// Recover captures a panic and then re-panics so SPIRE retains its normal
// crash semantics and Kubernetes can restart the process.
func (m *Monitor) Recover() {
	recovered := recover()
	if recovered == nil {
		return
	}
	if m.Enabled() {
		m.hub.Recover(recovered)
	}
	panic(recovered)
}

type sentryHook struct {
	hub *sentry.Hub
}

func (*sentryHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
	}
}

func (h *sentryHook) Fire(entry *logrus.Entry) error {
	if h == nil || h.hub == nil || !shouldCapture(entry) {
		return nil
	}

	hub := h.hub.Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentryLevel(entry.Level))
		scope.SetContext("log", logContext(entry))

		if entry.Message == healthFailureTransitionLog {
			check := fmt.Sprint(entry.Data["check"])
			scope.SetTag("health_check", check)
			scope.SetFingerprint([]string{"spire-health", check})
			hub.CaptureMessage(entry.Message)
			return
		}

		if err, ok := entry.Data[logrus.ErrorKey].(error); ok {
			hub.CaptureException(err)
			return
		}

		hub.CaptureMessage(entry.Message)
	})

	return nil
}

func shouldCapture(entry *logrus.Entry) bool {
	if entry == nil || entry.Message == repeatedHealthFailureLog {
		return false
	}
	if entry.Level == logrus.WarnLevel {
		return entry.Message == healthFailureTransitionLog
	}
	return entry.Level <= logrus.ErrorLevel
}

func sentryLevel(level logrus.Level) sentry.Level {
	switch level {
	case logrus.PanicLevel, logrus.FatalLevel:
		return sentry.LevelFatal
	case logrus.ErrorLevel:
		return sentry.LevelError
	case logrus.WarnLevel:
		return sentry.LevelWarning
	default:
		return sentry.LevelInfo
	}
}

func logContext(entry *logrus.Entry) sentry.Context {
	context := sentry.Context{
		"message": redactString(entry.Message),
	}
	for key, value := range entry.Data {
		if key == logrus.ErrorKey {
			continue
		}
		context[key] = scrubValue(key, value)
	}
	return context
}

func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}

	event.Message = redactString(event.Message)
	for i := range event.Exception {
		event.Exception[i].Value = redactString(event.Exception[i].Value)
	}
	for _, breadcrumb := range event.Breadcrumbs {
		if breadcrumb == nil {
			continue
		}
		breadcrumb.Message = redactString(breadcrumb.Message)
		breadcrumb.Data = scrubMap(breadcrumb.Data)
	}
	for name, context := range event.Contexts {
		event.Contexts[name] = scrubMap(context)
	}

	if event.Request != nil {
		event.Request.Cookies = ""
		event.Request.Data = ""
		for header := range event.Request.Headers {
			if isSensitiveKey(header) || sentry.IsSensitiveHeader(header) {
				event.Request.Headers[header] = filteredValue
			}
		}
	}

	return event
}

func scrubMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	for key, value := range values {
		values[key] = scrubValue(key, value)
	}
	return values
}

func scrubValue(key string, value any) any {
	if isSensitiveKey(key) {
		return filteredValue
	}

	switch typed := value.(type) {
	case string:
		return redactString(typed)
	case map[string]any:
		return scrubMap(typed)
	case logrus.Fields:
		return scrubMap(map[string]any(typed))
	case []any:
		for i := range typed {
			typed[i] = scrubValue("", typed[i])
		}
		return typed
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, sensitive := range []string{
		"authorization",
		"cookie",
		"credential",
		"dsn",
		"password",
		"passwd",
		"private_key",
		"secret",
		"token",
		"api_key",
	} {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}

func redactString(value string) string {
	return sensitiveValuePattern.ReplaceAllString(value, "$1$2"+filteredValue)
}

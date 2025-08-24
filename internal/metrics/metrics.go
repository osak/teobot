package metrics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	newrelic "github.com/newrelic/go-agent/v3/newrelic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// EndFunc is a function to end a started segment/transaction.
type EndFunc func()

// Provider defines a minimal metrics interface used by this app.
type Provider interface {
	Init(appName, license string) error
	StartTxn(ctx context.Context, name string) (context.Context, EndFunc)
	StartExternal(ctx context.Context, url string) EndFunc
	Count(name string, value float64)
	Observe(name string, value float64)
	NoticeError(ctx context.Context, err error)
}

// ----- New Relic provider -----

type newRelicProvider struct{ app *newrelic.Application }

func NewNewRelicProvider() Provider { return &newRelicProvider{} }

func (p *newRelicProvider) Init(appName, license string) error {
	a, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(license),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		return err
	}
	p.app = a
	return nil
}

func (p *newRelicProvider) StartTxn(ctx context.Context, name string) (context.Context, EndFunc) {
	if p.app == nil {
		return ctx, func() {}
	}
	txn := p.app.StartTransaction(name)
	ctx = newrelic.NewContext(ctx, txn)
	return ctx, func() { txn.End() }
}

func (p *newRelicProvider) StartExternal(ctx context.Context, url string) EndFunc {
	if txn := newrelic.FromContext(ctx); txn != nil {
		seg := newrelic.ExternalSegment{
			StartTime: newrelic.StartSegmentNow(txn),
			URL:       url,
		}
		return seg.End
	}
	return func() {}
}

func (p *newRelicProvider) Count(name string, value float64) {
	if p.app == nil {
		return
	}
	p.app.RecordCustomMetric(prefix(name), value)
}

func (p *newRelicProvider) Observe(name string, value float64) {
	// NR custom metrics aggregate stats; treat observe same as Count(value)
	p.Count(name, value)
}

func (p *newRelicProvider) NoticeError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.NoticeError(err)
	}
	p.Count("teobot/errors", 1)
}

// ----- No-op provider -----

type noopProvider struct{}

func NewNoopProvider() Provider                            { return &noopProvider{} }
func (n *noopProvider) Init(appName, license string) error { return nil }
func (n *noopProvider) StartTxn(ctx context.Context, name string) (context.Context, EndFunc) {
	return ctx, func() {}
}
func (n *noopProvider) StartExternal(ctx context.Context, url string) EndFunc { return func() {} }
func (n *noopProvider) Count(name string, value float64)                      {}
func (n *noopProvider) Observe(name string, value float64)                    {}
func (n *noopProvider) NoticeError(ctx context.Context, err error)            {}

// ----- Global/default management + helpers -----

type providerBox struct{ P Provider }

var (
	globalProv atomic.Value // providerBox
	slogLog    = slog.With("component", "metrics")
)

func init() { globalProv.Store(providerBox{P: NewNoopProvider()}) }

// SetGlobal swaps the global provider.
func SetGlobal(p Provider) { globalProv.Store(providerBox{P: p}) }
func getGlobal() Provider  { return globalProv.Load().(providerBox).P }

// Context-level override
type providerKeyType struct{}

var providerKey providerKeyType

func WithProvider(ctx context.Context, p Provider) context.Context {
	return context.WithValue(ctx, providerKey, p)
}

func from(ctx context.Context) Provider {
	if p, ok := ctx.Value(providerKey).(Provider); ok && p != nil {
		return p
	}
	return getGlobal()
}

// Init convenience: initialize NR provider if license given and set as global.
func Init(appName, licenseKey string) error {
	if licenseKey == "" {
		slogLog.Info("New Relic disabled: no license key provided")
		SetGlobal(NewNoopProvider())
		return nil
	}
	// Prefer OpenTelemetry provider that exports to New Relic OTLP ingest
	p := NewOTelProvider()
	if err := p.Init(appName, licenseKey); err != nil {
		return err
	}
	SetGlobal(p)
	slogLog.Info("OpenTelemetry initialized (exporting to New Relic)", "appName", appName)
	return nil
}

// Txn is a small interface with End(); returned by StartTxn helper for convenience.
type Txn interface{ End() }

type txnEnder struct{ end EndFunc }

func (t *txnEnder) End() {
	if t != nil && t.end != nil {
		t.end()
	}
}

// StartTxn starts a transaction using context or global provider.
func StartTxn(ctx context.Context, name string) (context.Context, Txn) {
	p := from(ctx)
	ctx2, end := p.StartTxn(ctx, name)
	if end == nil {
		return ctx2, nil
	}
	return ctx2, &txnEnder{end: end}
}

// StartExternalSegment starts an external segment with the active provider.
func StartExternalSegment(ctx context.Context, url string) func() {
	return from(ctx).StartExternal(ctx, url)
}

// RecordCount increments a custom metric by value.
func RecordCount(name string, value float64) { getGlobal().Count(name, value) }

// RecordCount1 increments a custom metric by 1.
func RecordCount1(name string) { RecordCount(name, 1) }

// RecordFloat records a float custom metric value.
func RecordFloat(name string, value float64) { getGlobal().Observe(name, value) }

// NoticeError records an error on the current transaction (if any).
func NoticeError(ctx context.Context, err error) { from(ctx).NoticeError(ctx, err) }

func prefix(name string) string {
	if len(name) >= 7 && name[:7] == "Custom/" {
		return name
	}
	return "Custom/" + name
}

// ----- OpenTelemetry provider (OTLP to New Relic) -----

type otelProvider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	mu       sync.Mutex
	counters map[string]metric.Int64Counter
	hists    map[string]metric.Float64Histogram
}

// NewOTelProvider creates an OTel provider that can be initialized to export to NR.
func NewOTelProvider() Provider {
	return &otelProvider{counters: map[string]metric.Int64Counter{}, hists: map[string]metric.Float64Histogram{}}
}

func (p *otelProvider) Init(appName, license string) error {
	// Resource with service.name
	res, err := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceNameKey.String(appName),
	))
	if err != nil {
		return err
	}

	// New Relic OTLP HTTP endpoint (4318) and headers
	// Endpoint: otlp.nr-data.net:4318 (HTTP)
	// Header: api-key: <license>

	// Trace exporter (HTTP)
	traceExp, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint("otlp.nr-data.net:4318"),
		otlptracehttp.WithHeaders(map[string]string{"api-key": license}),
	)
	if err != nil {
		return err
	}
	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(p.tracerProvider)
	p.tracer = otel.Tracer("teobot")

	// Metric exporter (HTTP)
	metricExp, err := otlpmetrichttp.New(context.Background(),
		otlpmetrichttp.WithEndpoint("otlp.nr-data.net:4318"),
		otlpmetrichttp.WithHeaders(map[string]string{"api-key": license}),
	)
	if err != nil {
		return err
	}
	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(p.meterProvider)
	p.meter = otel.Meter("teobot")

	return nil
}

func (p *otelProvider) StartTxn(ctx context.Context, name string) (context.Context, EndFunc) {
	if p.tracerProvider == nil {
		return ctx, func() {}
	}
	ctx, span := p.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	return ctx, func() { span.End() }
}

func (p *otelProvider) StartExternal(ctx context.Context, url string) EndFunc {
	if p.tracerProvider == nil {
		return func() {}
	}
	ctx2, span := p.tracer.Start(ctx, "external", trace.WithSpanKind(trace.SpanKindClient))
	_ = ctx2
	span.SetAttributes(attribute.String("http.url", url))
	return func() { span.End() }
}

func (p *otelProvider) Count(name string, value float64) {
	if p.meterProvider == nil {
		return
	}
	// Use an Int64Counter for counts; convert float to int64
	c := p.getCounter(name)
	c.Add(context.Background(), int64(value))
}

func (p *otelProvider) Observe(name string, value float64) {
	if p.meterProvider == nil {
		return
	}
	h := p.getHistogram(name)
	h.Record(context.Background(), value)
}

func (p *otelProvider) NoticeError(ctx context.Context, err error) {
	if err == nil || p.tracerProvider == nil {
		return
	}
	if span := trace.SpanFromContext(ctx); span != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", err.Error()))
	}
	// also increment an error counter metric
	p.Count("teobot/errors", 1)
}

func (p *otelProvider) getCounter(name string) metric.Int64Counter {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.counters[name]; ok {
		return c
	}
	c, _ := p.meter.Int64Counter(name)
	p.counters[name] = c
	return c
}

func (p *otelProvider) getHistogram(name string) metric.Float64Histogram {
	p.mu.Lock()
	defer p.mu.Unlock()
	if h, ok := p.hists[name]; ok {
		return h
	}
	h, _ := p.meter.Float64Histogram(name)
	p.hists[name] = h
	return h
}

// (gRPC creds helper removed in HTTP exporter mode)

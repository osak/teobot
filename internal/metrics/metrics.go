package metrics

import (
    "context"
    "log/slog"
    "sync/atomic"

    newrelic "github.com/newrelic/go-agent/v3/newrelic"
)

// EndFunc is a function to end a started segment/transaction.
type EndFunc func()

// Provider defines a minimal metrics interface used by this app.
type Provider interface {
    Init(appName, license string) error
    StartTxn(ctx context.Context, name string) (context.Context, EndFunc)
    StartExternal(ctx context.Context, url string) EndFunc
    Count(name string, value float64)
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

func NewNoopProvider() Provider { return &noopProvider{} }
func (n *noopProvider) Init(appName, license string) error                  { return nil }
func (n *noopProvider) StartTxn(ctx context.Context, name string) (context.Context, EndFunc) {
    return ctx, func() {}
}
func (n *noopProvider) StartExternal(ctx context.Context, url string) EndFunc { return func() {} }
func (n *noopProvider) Count(name string, value float64)                      {}
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
    p := NewNewRelicProvider()
    if err := p.Init(appName, licenseKey); err != nil {
        return err
    }
    SetGlobal(p)
    slogLog.Info("New Relic initialized", "appName", appName)
    return nil
}

// Txn is a small interface with End(); returned by StartTxn helper for convenience.
type Txn interface{ End() }

type txnEnder struct{ end EndFunc }

func (t *txnEnder) End() { if t != nil && t.end != nil { t.end() } }

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
func StartExternalSegment(ctx context.Context, url string) func() { return from(ctx).StartExternal(ctx, url) }

// RecordCount increments a custom metric by value.
func RecordCount(name string, value float64) { getGlobal().Count(name, value) }

// RecordCount1 increments a custom metric by 1.
func RecordCount1(name string) { RecordCount(name, 1) }

// RecordFloat records a float custom metric value.
func RecordFloat(name string, value float64) { RecordCount(name, value) }

// NoticeError records an error on the current transaction (if any).
func NoticeError(ctx context.Context, err error) { from(ctx).NoticeError(ctx, err) }

func prefix(name string) string {
    if len(name) >= 7 && name[:7] == "Custom/" {
        return name
    }
    return "Custom/" + name
}

package metrics

import (
    "context"
    "log/slog"

    newrelic "github.com/newrelic/go-agent/v3/newrelic"
)

var (
    app  *newrelic.Application
    log  = slog.With("component", "metrics")
)

// Init initializes New Relic application if a non-empty license key is provided.
// It is safe to call multiple times; only the first successful call takes effect.
func Init(appName, licenseKey string) error {
    if licenseKey == "" {
        log.Info("New Relic disabled: no license key provided")
        return nil
    }
    if app != nil {
        return nil
    }
    a, err := newrelic.NewApplication(
        newrelic.ConfigAppName(appName),
        newrelic.ConfigLicense(licenseKey),
        newrelic.ConfigDistributedTracerEnabled(true),
    )
    if err != nil {
        return err
    }
    // Non-blocking connect. Optionally wait briefly if you prefer.
    app = a
    log.Info("New Relic initialized", "appName", appName)
    return nil
}

// StartTxn starts a background transaction and returns a derived context carrying it.
// If NR is not initialized, returns the original context and a nil txn.
func StartTxn(ctx context.Context, name string) (context.Context, *newrelic.Transaction) {
    if app == nil {
        return ctx, nil
    }
    txn := app.StartTransaction(name)
    ctx = newrelic.NewContext(ctx, txn)
    return ctx, txn
}

// RecordCount increments a custom metric by value.
func RecordCount(name string, value float64) {
    if app == nil {
        return
    }
    // NR expects Custom/ prefix for custom metrics.
    app.RecordCustomMetric(prefix(name), value)
}

// RecordCount1 increments a custom metric by 1.
func RecordCount1(name string) { RecordCount(name, 1) }

// RecordFloat records a float custom metric value.
func RecordFloat(name string, value float64) { RecordCount(name, value) }

// NoticeError records an error on the current transaction (if any).
func NoticeError(ctx context.Context, err error) {
    if err == nil {
        return
    }
    if txn := newrelic.FromContext(ctx); txn != nil {
        txn.NoticeError(err)
    }
    RecordCount("teobot/errors", 1)
}

// StartExternalSegment starts an external segment linked to current txn.
// Returns a no-op end func if NR is disabled or txn missing.
func StartExternalSegment(ctx context.Context, url string) func() {
    if txn := newrelic.FromContext(ctx); txn != nil {
        seg := newrelic.ExternalSegment{
            StartTime: newrelic.StartSegmentNow(txn),
            URL:       url,
        }
        return func() { seg.End() }
    }
    return func() {}
}

func prefix(name string) string {
    if len(name) >= 7 && name[:7] == "Custom/" {
        return name
    }
    return "Custom/" + name
}

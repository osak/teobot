package metrics

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeProvider implements Provider and captures calls for assertions.
type fakeProvider struct {
	mu               sync.Mutex
	inited           bool
	counts           map[string]float64
	observations     map[string][]float64
	noticed          []error
	txnsStarted      int
	txnsEnded        int
	externalsStarted []string
	externalsEnded   int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{counts: map[string]float64{}, observations: map[string][]float64{}}
}

func (f *fakeProvider) Init(appName, license string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inited = true
	return nil
}
func (f *fakeProvider) StartTxn(ctx context.Context, name string) (context.Context, EndFunc) {
	f.mu.Lock()
	f.txnsStarted++
	f.mu.Unlock()
	return ctx, func() {
		f.mu.Lock()
		f.txnsEnded++
		f.mu.Unlock()
	}
}
func (f *fakeProvider) StartExternal(ctx context.Context, url string) EndFunc {
	f.mu.Lock()
	f.externalsStarted = append(f.externalsStarted, url)
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		f.externalsEnded++
		f.mu.Unlock()
	}
}
func (f *fakeProvider) Count(name string, value float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[name] += value
}
func (f *fakeProvider) Observe(name string, value float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observations[name] = append(f.observations[name], value)
	// Keep old tests simple by aggregating to counts too
	f.counts[name] += value
}
func (f *fakeProvider) NoticeError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	f.noticed = append(f.noticed, err)
	f.mu.Unlock()
}

func TestGlobalCount(t *testing.T) {
	fake := newFakeProvider()
	prev := getGlobal()
	SetGlobal(fake)
	t.Cleanup(func() { SetGlobal(prev) })

	RecordCount("a", 3)
	RecordCount1("a")
	RecordFloat("b", 2.5)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.counts["a"] != 4 {
		t.Fatalf("expected count a=4, got %v", fake.counts["a"])
	}
	if fake.counts["b"] != 2.5 {
		t.Fatalf("expected count b=2.5, got %v", fake.counts["b"])
	}
}

func TestContextOverrideNoticeError(t *testing.T) {
	// Ensure global is noop
	prev := getGlobal()
	SetGlobal(NewNoopProvider())
	t.Cleanup(func() { SetGlobal(prev) })

	fake := newFakeProvider()
	ctx := WithProvider(context.Background(), fake)

	NoticeError(ctx, errors.New("boom"))
	NoticeError(ctx, nil)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.noticed) != 1 {
		t.Fatalf("expected 1 noticed error, got %d", len(fake.noticed))
	}
}

func TestStartTxnAndExternal(t *testing.T) {
	fake := newFakeProvider()
	ctx := WithProvider(context.Background(), fake)

	ctx2, txn := StartTxn(ctx, "op")
	if txn == nil {
		t.Fatalf("expected non-nil txn")
	}
	txn.End()

	end := StartExternalSegment(ctx2, "http://example.com")
	end()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.txnsStarted != 1 || fake.txnsEnded != 1 {
		t.Fatalf("expected txn started=1 ended=1, got %d %d", fake.txnsStarted, fake.txnsEnded)
	}
	if len(fake.externalsStarted) != 1 || fake.externalsEnded != 1 {
		t.Fatalf("expected externals started=1 ended=1, got %d %d", len(fake.externalsStarted), fake.externalsEnded)
	}
}

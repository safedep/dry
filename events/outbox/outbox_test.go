package outbox

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- test doubles ---------------------------------------------------------

type testAdapter struct{ gdb *gorm.DB }

func (a testAdapter) GetDB() (*gorm.DB, error)  { return a.gdb, nil }
func (a testAdapter) GetConn() (*sql.DB, error) { return a.gdb.DB() }
func (a testAdapter) Migrate(models ...interface{}) error {
	return a.gdb.AutoMigrate(models...)
}
func (a testAdapter) Ping() error {
	c, err := a.gdb.DB()
	if err != nil {
		return err
	}
	return c.Ping()
}

func newStore(t *testing.T) testAdapter {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "outbox.db")), &gorm.Config{})
	require.NoError(t, err)
	a := testAdapter{gdb: gdb}
	require.NoError(t, Migrate(a))
	return a
}

type fakeDest struct {
	name       string
	mu         sync.Mutex
	delivered  [][]byte
	eventIDs   []string
	calls      int
	failFirst  int
	failAlways bool
}

func (f *fakeDest) Name() string { return f.name }

func (f *fakeDest) Publish(_ context.Context, req PublishRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failAlways || f.calls <= f.failFirst {
		return errors.New("boom")
	}
	f.delivered = append(f.delivered, req.Record)
	f.eventIDs = append(f.eventIDs, req.EventID)
	return nil
}

func (f *fakeDest) lastEventID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.eventIDs) == 0 {
		return ""
	}
	return f.eventIDs[len(f.eventIDs)-1]
}

func (f *fakeDest) deliveredCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.delivered) }
func (f *fakeDest) callCount() int      { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }

func newEvent(t *testing.T) proto.Message {
	t.Helper()
	obs := pkgregv1.PackageVersionObservationEvent_builder{
		PackageVersion: packagev1.PackageVersion_builder{
			Package: packagev1.Package_builder{Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM, Name: "x"}.Build(),
			Version: "1.0.0",
		}.Build(),
		Kind: pkgregv1.PackageVersionObservationEvent_KIND_PUBLISHED,
	}.Build()
	out, err := events.New(obs, events.WithSubject("pkg:npm/x"))
	require.NoError(t, err)
	return out
}

// --- construction ---------------------------------------------------------

func TestNew_Validation(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)

	_, err = New([]Destination{&fakeDest{name: "a"}, &fakeDest{name: "a"}})
	require.Error(t, err) // duplicate name

	_, err = New([]Destination{&fakeDest{name: ""}})
	require.Error(t, err) // empty name

	o, err := New([]Destination{&fakeDest{name: "a"}})
	require.NoError(t, err)
	assert.NotNil(t, o)
}

// --- Send (direct, no store) ---------------------------------------------

func TestSend_DirectNoStore(t *testing.T) {
	d := &fakeDest{name: "nats"}
	o, err := New([]Destination{d})
	require.NoError(t, err)

	evt := newEvent(t)
	meta, err := events.MetaOf(evt)
	require.NoError(t, err)

	require.NoError(t, o.Send(context.Background(), evt))
	assert.Equal(t, 1, d.deliveredCount())
	// The destination receives the real envelope event_id (not the feed FQN).
	assert.Equal(t, meta.GetEventId(), d.lastEventID())
	assert.NotEmpty(t, d.lastEventID())
}

func TestSend_RejectsMissingEventID(t *testing.T) {
	d := &fakeDest{name: "nats"}
	o, err := New([]Destination{d})
	require.NoError(t, err)

	// A well-typed event with NO envelope stamped (event_id empty) must be
	// rejected before publish — it would break consumer dedupe.
	raw := pkgregv1.PackageVersionObservationEvent_builder{
		Kind: pkgregv1.PackageVersionObservationEvent_KIND_PUBLISHED,
	}.Build()

	err = o.Send(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_id")
	assert.Equal(t, 0, d.callCount())
}

func TestSend_DirectPartialError(t *testing.T) {
	healthy := &fakeDest{name: "s2"}
	broken := &fakeDest{name: "nats", failAlways: true}
	o, err := New([]Destination{healthy, broken})
	require.NoError(t, err)

	err = o.Send(context.Background(), newEvent(t))
	require.Error(t, err)                        // broken surfaced
	assert.Equal(t, 1, healthy.deliveredCount()) // healthy still delivered
}

// --- Emit (transactional) -------------------------------------------------

func TestEmit_RequiresStore(t *testing.T) {
	o, err := New([]Destination{&fakeDest{name: "a"}})
	require.NoError(t, err)
	require.ErrorIs(t, o.Emit(context.Background(), nil, newEvent(t)), ErrNoStore)
}

func TestEmit_TransactionalCommit(t *testing.T) {
	store := newStore(t)
	d := &fakeDest{name: "s2"}
	o, err := New([]Destination{d}, WithStore(store))
	require.NoError(t, err)

	err = store.gdb.Transaction(func(tx *gorm.DB) error {
		return o.Emit(context.Background(), tx, newEvent(t))
	})
	require.NoError(t, err)

	var records, deliveries int64
	store.gdb.Model(&Record{}).Count(&records)
	store.gdb.Model(&Delivery{}).Count(&deliveries)
	assert.Equal(t, int64(1), records)
	assert.Equal(t, int64(1), deliveries)
	assert.Equal(t, 0, d.callCount(), "Emit persists only; the drain publishes")
}

func TestEmit_TransactionalRollback(t *testing.T) {
	store := newStore(t)
	o, err := New([]Destination{&fakeDest{name: "s2"}}, WithStore(store))
	require.NoError(t, err)

	sentinel := errors.New("business write failed")
	err = store.gdb.Transaction(func(tx *gorm.DB) error {
		if e := o.Emit(context.Background(), tx, newEvent(t)); e != nil {
			return e
		}
		return sentinel // roll back the whole tx
	})
	require.ErrorIs(t, err, sentinel)

	var records int64
	store.gdb.Model(&Record{}).Count(&records)
	assert.Equal(t, int64(0), records, "rollback discards the event atomically")
}

// --- Send (buffered) + drain ----------------------------------------------

func TestSend_BufferedPersistsThenDrains(t *testing.T) {
	store := newStore(t)
	d := &fakeDest{name: "s2"}
	o, err := New([]Destination{d}, WithStore(store))
	require.NoError(t, err)

	require.NoError(t, o.Send(context.Background(), newEvent(t)))

	// Buffered, not yet published.
	assert.Equal(t, 0, d.callCount())
	var pending int64
	store.gdb.Model(&Delivery{}).Where("published_at IS NULL").Count(&pending)
	assert.Equal(t, int64(1), pending)

	// Drain publishes and marks delivered.
	n, err := o.drainOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, d.deliveredCount())

	var delivered int64
	store.gdb.Model(&Record{}).Where("delivered_at IS NOT NULL").Count(&delivered)
	assert.Equal(t, int64(1), delivered)
}

func TestRun_NoOpWithoutStore(t *testing.T) {
	o, err := New([]Destination{&fakeDest{name: "a"}})
	require.NoError(t, err)
	require.NoError(t, o.Run(context.Background())) // returns immediately, no store
}

// --- per-destination poison & isolation (§8) ------------------------------

func TestDrain_PerDestinationPoisonAndIsolation(t *testing.T) {
	store := newStore(t)
	healthy := &fakeDest{name: "s2"}
	broken := &fakeDest{name: "nats", failAlways: true}
	o, err := New([]Destination{healthy, broken}, WithStore(store), WithMaxAttempts(2))
	require.NoError(t, err)

	require.NoError(t, o.Send(context.Background(), newEvent(t)))

	// Drain enough times to exhaust the broken destination's attempts.
	for i := 0; i < 3; i++ {
		_, err := o.drainOnce(context.Background())
		require.NoError(t, err)
	}

	// Healthy delivered exactly once and was NOT re-sent on later polls.
	assert.Equal(t, 1, healthy.deliveredCount())
	assert.Equal(t, 1, healthy.callCount(), "a healthy destination is not re-sent when a sibling fails")

	// Broken never delivered and is poisoned (failed_at set), isolated.
	assert.Equal(t, 0, broken.deliveredCount())
	var poisoned int64
	store.gdb.Model(&Delivery{}).Where("destination = ? AND failed_at IS NOT NULL", "nats").Count(&poisoned)
	assert.Equal(t, int64(1), poisoned)

	// Record is resolved (healthy published, broken poisoned) → delivered_at set.
	var delivered int64
	store.gdb.Model(&Record{}).Where("delivered_at IS NOT NULL").Count(&delivered)
	assert.Equal(t, int64(1), delivered)
}

func TestDrain_TransientRetryThenSucceeds(t *testing.T) {
	store := newStore(t)
	flaky := &fakeDest{name: "s2", failFirst: 2} // fail twice, then succeed
	o, err := New([]Destination{flaky}, WithStore(store), WithMaxAttempts(5))
	require.NoError(t, err)

	require.NoError(t, o.Send(context.Background(), newEvent(t)))

	// First two drains fail transiently (no poison), third succeeds.
	for i := 0; i < 3; i++ {
		_, err := o.drainOnce(context.Background())
		require.NoError(t, err)
	}

	assert.Equal(t, 1, flaky.deliveredCount())
	var poisoned int64
	store.gdb.Model(&Delivery{}).Where("failed_at IS NOT NULL").Count(&poisoned)
	assert.Equal(t, int64(0), poisoned, "transient failures below maxAttempts must not poison")
}

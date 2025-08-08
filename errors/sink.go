package errors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/safedep/dry/log"
)

// sinkMeta holds metadata for a sink, such as an error code.
// These are first-class values that can be passed to the sink
type sinkMeta struct {
	errorCode string
	values    map[string]any
}

type sinkMetaFn func(*sinkMeta)

func WithErrorCode(code string) sinkMetaFn {
	return func(m *sinkMeta) {
		m.errorCode = code
	}
}

// WithValue adds a value to the sink metadata.
// This can be used to pass additional context
func WithValue(key string, value any) sinkMetaFn {
	return func(m *sinkMeta) {
		if m.values == nil {
			m.values = make(map[string]any)
		}

		m.values[key] = value
	}
}

// WithValues sets multiple values in the sink metadata.
// It overrides any existing values.
func WithValues(values map[string]any) sinkMetaFn {
	return func(m *sinkMeta) {
		m.values = values
	}
}

type SinkConfig struct {
	MaxBatchSize int
	CloseTimeout time.Duration
}

func DefaultSinkConfig() SinkConfig {
	return SinkConfig{
		MaxBatchSize: 100,
		CloseTimeout: 5 * time.Second,
	}
}

// Sink is an interface for handling errors.
// This is like the kitchen sink to safely throw errors that
// the application want to log, report etc. The handling of the
// errors is on a best effort basis, meaning that the sink
// may not be able to handle all errors. This is the cost of
// handling safely without blocking the application.
type Sink interface {
	// Handle processes an error with optional metadata.
	// This method is non-blocking and should not
	// block the application. The error handling is on a
	// best effort basis, meaning that the sink may not
	// be able to handle all errors, or may drop some errors
	// if the sink is overloaded.
	Handle(err error, metaFns ...sinkMetaFn)

	// Close is called to flush any remaining errors and
	// release resources. No further calls to Handle
	// should be made after Close.
	Close() error
}

type errorWithMeta struct {
	err  error
	meta sinkMeta
}

// sinkHandleSingleError is the signature for internal drivers that
// actually connects the sink batcher to the sink.
type sinkHandleSingleError func(*errorWithMeta) error

// sinkHandleClose is the signature for internal drivers that
// close the sink and flush any remaining errors.
type sinkHandleClose func() error

type sinkErrorBatcher struct {
	errors        chan errorWithMeta
	driverHandler sinkHandleSingleError
	driverCloser  sinkHandleClose
	config        SinkConfig
	m             sync.RWMutex
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	closed        bool
}

func newSinkErrorBatcher(config SinkConfig,
	driverHandler sinkHandleSingleError,
	driverCloser sinkHandleClose) (*sinkErrorBatcher, error) {
	if driverHandler == nil {
		return nil, fmt.Errorf("driverHandler cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sb := &sinkErrorBatcher{
		errors:        make(chan errorWithMeta, config.MaxBatchSize),
		config:        config,
		driverHandler: driverHandler,
		driverCloser:  driverCloser,
		ctx:           ctx,
		cancel:        cancel,
	}

	err := sb.startInternal()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start sink error batcher: %w", err)
	}

	return sb, nil
}

func (b *sinkErrorBatcher) Handle(err error, metaFns ...sinkMetaFn) {
	if err == nil {
		return
	}

	b.m.RLock()
	defer b.m.RUnlock()

	if b.closed {
		return
	}

	meta := sinkMeta{}
	for _, fn := range metaFns {
		fn(&meta)
	}

	select {
	case b.errors <- errorWithMeta{err: err, meta: meta}:
		// Successfully added the error to the channel.
	case <-b.ctx.Done():
		// Sink is being closed, drop the error.
		return
	default:
		// The channel is full, meaning the sink is overloaded.
		log.Warnf("sink error batcher is overloaded, dropping error: %v", err)
	}
}

func (b *sinkErrorBatcher) Close() error {
	b.m.Lock()
	defer b.m.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Signal the goroutine to stop
	b.cancel()

	// Close the channel to stop accepting new errors
	close(b.errors)

	// Wait for the processing goroutine to finish with a timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutine finished gracefully
	case <-time.After(b.config.CloseTimeout):
		log.Warnf("sink error batcher shutdown timeout after %v, some errors may be lost", b.config.CloseTimeout)
	}

	// Close the driver after processing is complete
	if b.driverCloser != nil {
		if err := b.driverCloser(); err != nil {
			return fmt.Errorf("failed to close sink driver: %w", err)
		}
	}

	return nil
}

func (b *sinkErrorBatcher) startInternal() error {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		handleError := func(em *errorWithMeta, context string) {
			err := b.driverHandler(em)
			if err != nil {
				log.Errorf("failed to handle error with sink driver%s: %v", context, err)
			}
		}

		for {
			select {
			case em, ok := <-b.errors:
				if !ok {
					// Channel is closed, exit the goroutine
					return
				}

				handleError(&em, "")
			case <-b.ctx.Done():
				// Context cancelled, process remaining errors and exit
				for {
					select {
					case em, ok := <-b.errors:
						if !ok {
							return
						}

						handleError(&em, " during shutdown")
					default:
						// No more errors to process
						return
					}
				}
			}
		}
	}()

	return nil
}

package history

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultMaxPending    = 8192
	defaultFlushBatch    = 512
	defaultFlushInterval = 500 * time.Millisecond
	defaultPruneInterval = 5 * time.Second
)

// AsyncWriter preserves selected samples and their task membership in FIFO
// order. Capacity includes the in-flight transaction; failures cannot evict
// older samples in favour of newer ones. Overflow is explicit and counted.
type AsyncWriter struct {
	store        *Store
	onError      func(error)
	mu           sync.Mutex
	pending      []Sample
	inFlight     int
	closeErr     error
	maxPending   int
	dropped      uint64
	lastReport   string
	lastReportAt time.Time
	closed       bool
	stop         chan struct{}
	wake         chan struct{}
	done         chan struct{}
}

type WriterStatus struct {
	Pending int
	Dropped uint64
}

func NewAsyncWriter(store *Store, onError func(error)) *AsyncWriter {
	writer := &AsyncWriter{
		store:      store,
		onError:    onError,
		maxPending: defaultMaxPending,
		stop:       make(chan struct{}),
		wake:       make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
	go writer.run()
	return writer
}

func (w *AsyncWriter) Enqueue(samples []Sample) {
	if w == nil || len(samples) == 0 {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	var dropped uint64
	belowBatch := len(w.pending) < defaultFlushBatch
	for _, sample := range samples {
		if len(w.pending)+w.inFlight >= w.maxPending {
			w.dropped++
			dropped++
			continue
		}
		sample.TaskKeys = append([]string(nil), sample.TaskKeys...)
		w.pending = append(w.pending, sample)
	}
	w.publishStatusLocked()
	wake := belowBatch && len(w.pending) >= defaultFlushBatch
	w.mu.Unlock()
	if wake {
		w.signalFlush()
	}
	if dropped > 0 {
		w.report(fmt.Errorf("历史写入队列已满，丢弃 %d 条新采样；请检查磁盘状态或降低保存频率", dropped))
	}
}

func (w *AsyncWriter) signalFlush() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *AsyncWriter) flushReadyBatch() {
	if w.flush() && w.Status().Pending >= defaultFlushBatch {
		w.signalFlush()
	}
}

func (w *AsyncWriter) publishStatusLocked() {
	if w.store != nil {
		w.store.queuePending.Store(int64(len(w.pending) + w.inFlight))
		w.store.queueDropped.Store(w.dropped)
	}
}

func (w *AsyncWriter) Status() WriterStatus {
	if w == nil {
		return WriterStatus{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return WriterStatus{Pending: len(w.pending) + w.inFlight, Dropped: w.dropped}
}

func (w *AsyncWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.stop)
	}
	w.mu.Unlock()
	<-w.done
	if w.store != nil {
		if err := w.store.Close(); err != nil {
			return err
		}
	}
	return w.closeErr
}

func (w *AsyncWriter) run() {
	flushTicker := time.NewTicker(defaultFlushInterval)
	pruneTicker := time.NewTicker(defaultPruneInterval)
	defer flushTicker.Stop()
	defer pruneTicker.Stop()
	defer close(w.done)
	for {
		select {
		case <-flushTicker.C:
			w.flushReadyBatch()
		case <-w.wake:
			w.flushReadyBatch()
		case <-pruneTicker.C:
			w.prune()
		case <-w.stop:
			for w.Status().Pending > 0 {
				if !w.flush() {
					w.closeErr = fmt.Errorf("历史存储关闭时仍有 %d 条采样未落盘", w.Status().Pending)
					w.report(w.closeErr)
					break
				}
			}
			w.prune()
			return
		}
	}
}

func (w *AsyncWriter) flush() bool {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return true
	}
	count := min(len(w.pending), defaultFlushBatch)
	samples := append([]Sample(nil), w.pending[:count]...)
	clear(w.pending[:count])
	w.pending = w.pending[count:]
	w.inFlight = count
	w.publishStatusLocked()
	w.mu.Unlock()
	if w.store == nil {
		w.requeue(samples)
		return false
	}
	if err := w.store.AppendBatch(samples); err != nil {
		w.requeue(samples)
		if err != ErrDisabled {
			w.report(err)
		}
		return false
	}
	w.mu.Lock()
	w.inFlight = 0
	w.publishStatusLocked()
	w.mu.Unlock()
	return true
}

func (w *AsyncWriter) requeue(samples []Sample) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(samples, w.pending...)
	w.inFlight = 0
	w.publishStatusLocked()
}

func (w *AsyncWriter) prune() {
	if w.store == nil {
		return
	}
	if err := w.store.Prune(); err != nil && err != ErrDisabled {
		w.report(err)
	}
}

func (w *AsyncWriter) report(err error) {
	if w.onError == nil || err == nil {
		return
	}
	w.mu.Lock()
	now := time.Now()
	text := err.Error()
	if text == w.lastReport && now.Sub(w.lastReportAt) < 30*time.Second {
		w.mu.Unlock()
		return
	}
	w.lastReport, w.lastReportAt = text, now
	w.mu.Unlock()
	w.onError(err)
}

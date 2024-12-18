package worker

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/HiroseKakeru/mqtt-speedtest/pkg/mqtt"
)

type Sender interface {
	Send() error
	PayloadOfferLength() int // for logging
	PayloadSize() int        // for logging
}

type ShippingWorker struct {
	jobQueue chan Sender
	quit     chan bool
	wg       *sync.WaitGroup
	total    *atomic.Int64 // for logging
	ID       int           // for logging
}

func (w *ShippingWorker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case job, ok := <-w.jobQueue:
				if !ok {
					slog.Debug("channel closed", "worker_id", w.ID)
					return
				}
				if job != nil {
					slog.Debug("sending payload", "worker_id", w.ID, "payload_offer_length", job.PayloadOfferLength(), "payload_bytes", job.PayloadSize())
					if err := job.Send(); err != nil {
						slog.Error("send payload failed", "error", err)
					}
					w.total.Add(int64(job.PayloadOfferLength()))
				}
			case <-w.quit:
				slog.Debug("worker stopped", "worker_id", w.ID)
				return
			}
		}
	}()
}

func (w *ShippingWorker) Stop() {
	go func() {
		slog.Debug("stopping worker", "worker_id", w.ID)
		w.quit <- true
	}()
}

func NewShippingWorker(id int, wg *sync.WaitGroup, jobCh chan Sender, total *atomic.Int64) *ShippingWorker {
	return &ShippingWorker{
		ID:       id,
		jobQueue: jobCh,
		quit:     make(chan bool),
		wg:       wg,
		total:    total,
	}
}

type Dispatcher struct {
	wg          *sync.WaitGroup
	TotalRecord *atomic.Int64 // for logging
	WorkerPool  []*ShippingWorker
	maxWorkers  int
}

func NewDispatcher() *Dispatcher {
	totalRecord := &atomic.Int64{}
	totalRecord.Store(0)
	maxWorkers := mqtt.ClientCount
	return &Dispatcher{
		WorkerPool:  []*ShippingWorker{},
		maxWorkers:  maxWorkers,
		wg:          &sync.WaitGroup{},
		TotalRecord: totalRecord,
	}
}

func (d *Dispatcher) Run(ch chan Sender) {
	// starting n number of workers
	for i := 0; i < d.maxWorkers; i++ {
		worker := NewShippingWorker(i, d.wg, ch, d.TotalRecord)
		worker.Start()
		d.WorkerPool = append(d.WorkerPool, worker)
	}
}

func (d *Dispatcher) Wait() {
	d.wg.Wait()
}

func (d *Dispatcher) Stop() {
	for _, worker := range d.WorkerPool {
		worker.Stop()
	}
	d.wg.Wait()
}

type Publisher interface {
	Publish([]byte, string) error
}

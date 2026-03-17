package overlay

import "time"

// CollectFunc returns the current overlay state to be written to shared memory.
// Called on each poll tick. The caller is responsible for reading GPU/CPU metrics
// and evaluating alerts — the collector only handles timing and IPC writes.
type CollectFunc func() SharedState

// Collector periodically calls a collect function and writes the resulting
// state to the overlay IPC shared memory file.
type Collector struct {
	ipc      *IPCFile
	collect  CollectFunc
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewCollector creates a collector that writes overlay state at the given interval.
func NewCollector(ipc *IPCFile, collect CollectFunc, interval time.Duration) *Collector {
	return &Collector{
		ipc:      ipc,
		collect:  collect,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the collection loop in a goroutine. Call Stop to end it.
func (c *Collector) Start() {
	go c.run()
}

// Stop signals the collector to stop and waits for it to finish.
func (c *Collector) Stop() {
	close(c.stop)
	<-c.done
}

func (c *Collector) run() {
	defer close(c.done)

	// Write immediately on start
	state := c.collect()
	c.ipc.WriteState(&state)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			state := c.collect()
			c.ipc.WriteState(&state)
		}
	}
}

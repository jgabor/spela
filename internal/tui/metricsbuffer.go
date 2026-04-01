package tui

// MetricsBuffer is a fixed-capacity ring buffer for float64 metric samples.
// It stores the most recent N values in chronological order, overwriting the
// oldest sample when full. This is used to feed sparkline renderers with
// historical metric data.
type MetricsBuffer struct {
	data   []float64
	head   int // index where the next write goes (also oldest element when full)
	length int // number of valid elements (0 to cap)
}

// NewMetricsBuffer creates a ring buffer with the given maximum capacity.
// Panics if capacity is zero or negative.
func NewMetricsBuffer(capacity int) *MetricsBuffer {
	if capacity <= 0 {
		panic("MetricsBuffer: capacity must be positive")
	}
	return &MetricsBuffer{
		data: make([]float64, capacity),
	}
}

// Push adds a sample to the buffer. When the buffer is full, the oldest
// sample is overwritten.
func (b *MetricsBuffer) Push(value float64) {
	b.data[b.head] = value
	b.head = (b.head + 1) % len(b.data)
	if b.length < len(b.data) {
		b.length++
	}
}

// Values returns all valid samples in chronological order (oldest first).
// The returned slice is a new allocation safe for the caller to modify.
func (b *MetricsBuffer) Values() []float64 {
	if b.length == 0 {
		return nil
	}
	result := make([]float64, b.length)
	capacity := len(b.data)
	if b.length < capacity {
		// Not full yet: data starts at index 0.
		copy(result, b.data[:b.length])
	} else {
		// Full: head points to the oldest element.
		firstPart := capacity - b.head
		copy(result, b.data[b.head:])
		copy(result[firstPart:], b.data[:b.head])
	}
	return result
}

// Len returns the number of valid samples currently in the buffer.
func (b *MetricsBuffer) Len() int {
	return b.length
}

// Cap returns the maximum number of samples the buffer can hold.
func (b *MetricsBuffer) Cap() int {
	return len(b.data)
}

// Last returns the most recently pushed value. The second return value is
// false if the buffer is empty.
func (b *MetricsBuffer) Last() (float64, bool) {
	if b.length == 0 {
		return 0, false
	}
	// head points to the next write slot; the most recent value is at head-1.
	idx := (b.head - 1 + len(b.data)) % len(b.data)
	return b.data[idx], true
}

package util

import (
	"sync/atomic"
)

// ParamUpdate represents a parameter change from UI to audio thread
type ParamUpdate struct {
	TargetID   int     // Which synth/effect to update
	ParameterID int    // Which parameter to update
	Value      float32 // New value
}

// RingBuffer is a lock-free single-producer single-consumer ring buffer
// for passing parameter updates from UI thread to audio thread
type RingBuffer struct {
	data     []ParamUpdate
	capacity int
	readPos  atomic.Uint64
	writePos atomic.Uint64
}

// NewRingBuffer creates a new ring buffer with the specified capacity
// Capacity must be a power of 2 for efficient modulo operations
func NewRingBuffer(capacity int) *RingBuffer {
	// Round up to next power of 2
	if capacity&(capacity-1) != 0 {
		capacity = nextPowerOf2(capacity)
	}

	return &RingBuffer{
		data:     make([]ParamUpdate, capacity),
		capacity: capacity,
	}
}

// Write adds a parameter update to the ring buffer
// Returns true if successful, false if buffer is full
// This should only be called from the UI thread
func (rb *RingBuffer) Write(update ParamUpdate) bool {
	writePos := rb.writePos.Load()
	readPos := rb.readPos.Load()

	// Check if buffer is full
	// We keep one slot empty to distinguish full from empty
	nextWritePos := (writePos + 1) % uint64(rb.capacity)
	if nextWritePos == readPos {
		return false // Buffer full
	}

	// Write data
	rb.data[writePos] = update

	// Update write position (release semantics)
	rb.writePos.Store(nextWritePos)

	return true
}

// Read removes and returns a parameter update from the ring buffer
// Returns the update and true if successful, zero value and false if buffer is empty
// This should only be called from the audio thread
func (rb *RingBuffer) Read() (ParamUpdate, bool) {
	readPos := rb.readPos.Load()
	writePos := rb.writePos.Load()

	// Check if buffer is empty
	if readPos == writePos {
		return ParamUpdate{}, false
	}

	// Read data
	update := rb.data[readPos]

	// Update read position (release semantics)
	nextReadPos := (readPos + 1) % uint64(rb.capacity)
	rb.readPos.Store(nextReadPos)

	return update, true
}

// Available returns the number of updates available to read
func (rb *RingBuffer) Available() int {
	readPos := rb.readPos.Load()
	writePos := rb.writePos.Load()

	if writePos >= readPos {
		return int(writePos - readPos)
	}
	return rb.capacity - int(readPos-writePos)
}

// Clear empties the ring buffer
func (rb *RingBuffer) Clear() {
	rb.readPos.Store(0)
	rb.writePos.Store(0)
}

// nextPowerOf2 returns the next power of 2 >= n
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

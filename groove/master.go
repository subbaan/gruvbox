package groove

import "math"

// The master bus sits after the track mix, for live playback and export alike.
// Tracks are mixed at .45 × level each, which leaves generated grooves peaking
// around -10 dBFS (RMS near -27). The bus adds fixed make-up gain and then a
// look-ahead peak limiter with a -1 dBFS ceiling, so grooves come out at a
// normal listening level without hard clipping.
const (
	MasterGainDB       = 9.0
	MasterCeilingDB    = -1.0
	masterLookahead    = 64 // frames (1.45 ms); the bus delays audio by masterLookahead-1
	masterLatency      = masterLookahead - 1
	masterReleaseSecs  = .12
	masterMinGainLimit = 1e-6
)

// masterBus is audio-thread state: fixed arrays, no allocation, no locks.
//
// For each frame it works out the gain that would keep the louder channel
// under the ceiling, takes the lowest such gain over a look-ahead window (a
// sliding minimum), lets it recover exponentially once the window has passed,
// and smooths it with a moving average as long as the window. Audio is delayed
// by the window minus one, so every gain in the average already covers the
// sample it is applied to: the ramp down finishes exactly as a peak arrives
// and the output never exceeds the ceiling.
type masterBus struct {
	gain, ceiling, release float32
	audio                  [masterLatency * 2]float32
	audioPos               int
	box                    [masterLookahead]float32
	boxPos                 int
	boxSum                 float64
	minGain                [masterLookahead]float32 // sliding-minimum deque, ring buffer
	minFrame               [masterLookahead]uint64
	minHead, minCount      int
	frame                  uint64
	held                   float32
	reduction              float32 // lowest gain applied since the last reset, for tests and meters
}

func newMasterBus() masterBus {
	m := masterBus{
		gain:    float32(math.Pow(10, MasterGainDB/20)),
		ceiling: float32(math.Pow(10, MasterCeilingDB/20)),
		release: float32(1 - math.Exp(-1/(masterReleaseSecs*SampleRate))),
	}
	m.reset()
	return m
}

func (m *masterBus) reset() {
	clear(m.audio[:])
	m.audioPos, m.boxPos, m.minHead, m.minCount, m.frame = 0, 0, 0, 0, 0
	for i := range m.box {
		m.box[i] = 1
	}
	m.boxSum = masterLookahead
	m.held, m.reduction = 1, 1
}

// process applies make-up gain and limiting to interleaved stereo in place.
// The output lags the input by masterLatency frames.
func (m *masterBus) process(buffer []float32) {
	for i := 0; i+1 < len(buffer); i += 2 {
		left, right := buffer[i]*m.gain, buffer[i+1]*m.gain
		peak := max(left, -left, right, -right)
		want := float32(1)
		if peak > m.ceiling {
			want = max(masterMinGainLimit, m.ceiling/peak)
		}
		// Sliding minimum of want over the last masterLookahead frames. Expire
		// first so the deque never holds more than masterLookahead entries.
		if m.minCount > 0 && m.frame-m.minFrame[m.minHead] >= masterLookahead {
			m.minHead = (m.minHead + 1) % masterLookahead
			m.minCount--
		}
		for m.minCount > 0 && m.minGain[(m.minHead+m.minCount-1)%masterLookahead] >= want {
			m.minCount--
		}
		tail := (m.minHead + m.minCount) % masterLookahead
		m.minGain[tail], m.minFrame[tail] = want, m.frame
		m.minCount++
		m.frame++
		m.held = min(m.minGain[m.minHead], m.held+(1-m.held)*m.release)
		m.boxSum += float64(m.held - m.box[m.boxPos])
		m.box[m.boxPos] = m.held
		m.boxPos++
		if m.boxPos == masterLookahead {
			m.boxPos = 0
			// Re-add from scratch once per window so rounding never builds up.
			sum := 0.
			for _, g := range m.box {
				sum += float64(g)
			}
			m.boxSum = sum
		}
		g := float32(m.boxSum / masterLookahead)
		m.reduction = min(m.reduction, g)
		j := m.audioPos * 2
		outLeft, outRight := m.audio[j]*g, m.audio[j+1]*g
		m.audio[j], m.audio[j+1] = left, right
		m.audioPos++
		if m.audioPos == masterLatency {
			m.audioPos = 0
		}
		buffer[i] = max(-m.ceiling, min(m.ceiling, outLeft))
		buffer[i+1] = max(-m.ceiling, min(m.ceiling, outRight))
	}
}

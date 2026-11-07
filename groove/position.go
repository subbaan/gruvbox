package groove

// The renderer may run ahead to fill audio buffers. Keep a bounded atomic
// history of actual step boundaries so the UI can follow consumed audio without
// changing source-buffer sizes or reading mutable DSP state.
func (v *voice) recordPosition(frame uint64, index int) {
	v.recordPatternPosition(frame, index, 0)
}
func (v *voice) recordPatternPosition(frame uint64, index, pattern int) {
	seq := v.positionSequence.Load() + 1
	v.positionHistory[(seq-1)%positionHistorySize].Store((frame+1)<<9 | uint64(pattern)<<6 | uint64(index))
	v.positionSequence.Store(seq)
}

func (v *voice) positionAt(frame int64) int {
	if frame < 0 {
		return -1
	}
	seq := v.positionSequence.Load()
	for n := uint64(0); n < min(seq, positionHistorySize); n++ {
		packed := v.positionHistory[(seq-1-n)%positionHistorySize].Load()
		if packed != 0 && int64((packed>>9)-1) <= frame {
			return int(packed & 63)
		}
	}
	return -1
}

func consumedFrame(readBytes uint64, queuedBytes int) int64 {
	return int64(readBytes/4) - int64(queuedBytes/4) - 1
}

func (p *Player) audibleFrame() int64 {
	// BufferedSize reports source audio not yet handed to the device. Sampling
	// it on the UI thread avoids any lock or device query in our audio callback.
	return consumedFrame(p.readBytes.Load(), p.device.BufferedSize())
}

func (v *voice) patternAt(frame int64) int {
	if frame < 0 {
		return -1
	}
	seq := v.positionSequence.Load()
	for n := uint64(0); n < min(seq, positionHistorySize); n++ {
		packed := v.positionHistory[(seq-1-n)%positionHistorySize].Load()
		if packed != 0 && int64((packed>>9)-1) <= frame {
			return int((packed >> 6) & 7)
		}
	}
	return -1
}

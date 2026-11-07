package sequencer

// PatternResolution defines the timing resolution of a pattern
type PatternResolution int

const (
	// Resolution16th means each step is a 16th note
	Resolution16th PatternResolution = iota
	// Resolution32nd means each step is a 32nd note
	Resolution32nd
	// Resolution8th means each step is an 8th note
	Resolution8th
)

// Pattern represents a sequence of steps
type Pattern struct {
	// Steps in the pattern
	Steps []Step

	// Length is the number of active steps (may be less than len(Steps))
	Length int

	// Resolution defines the timing of each step
	Resolution PatternResolution

	// Name of the pattern (optional)
	Name string

	// SoundParams stores sound settings for this pattern
	// Keys depend on instrument type:
	// TB-303: waveform, tune, cutoff, resonance, envmod, decay, accent, slidetime, lfo_enabled, lfo_target
	// Kick: tune, decay, pitchdecay, drive, click
	// Snare: tune, snap, tonedecay, noisedecay, tonemix
	// HiHat: decay, tone, noisemix
	SoundParams map[string]float32
}

// NewPattern creates a new pattern with the specified length
func NewPattern(length int) *Pattern {
	if length < 1 {
		length = 16
	}
	if length > 128 {
		length = 128
	}

	steps := make([]Step, length)
	for i := range steps {
		steps[i] = NewStep()
	}

	return &Pattern{
		Steps:       steps,
		Length:      length,
		Resolution:  Resolution16th,
		Name:        "",
		SoundParams: make(map[string]float32),
	}
}

// NewPattern16 creates a standard 16-step pattern
func NewPattern16() *Pattern {
	return NewPattern(16)
}

// NewPattern32 creates a 32-step pattern
func NewPattern32() *Pattern {
	return NewPattern(32)
}

// GetStep returns the step at the given index
// Returns nil if index is out of bounds
func (p *Pattern) GetStep(index int) *Step {
	if index < 0 || index >= len(p.Steps) {
		return nil
	}
	return &p.Steps[index]
}

// SetStep sets the step at the given index
func (p *Pattern) SetStep(index int, step Step) {
	if index >= 0 && index < len(p.Steps) {
		p.Steps[index] = step
	}
}

// ToggleStep toggles a step on/off
func (p *Pattern) ToggleStep(index int) {
	if step := p.GetStep(index); step != nil {
		step.Toggle()
	}
}

// ClearStep clears a step
func (p *Pattern) ClearStep(index int) {
	if step := p.GetStep(index); step != nil {
		step.Clear()
	}
}

// ClearAll clears all steps in the pattern
func (p *Pattern) ClearAll() {
	for i := range p.Steps {
		p.Steps[i].Clear()
	}
}

// SetLength changes the pattern length
func (p *Pattern) SetLength(length int) {
	if length < 1 {
		length = 1
	}
	if length > 128 {
		length = 128
	}

	// If expanding, add new steps
	if length > len(p.Steps) {
		for len(p.Steps) < length {
			p.Steps = append(p.Steps, NewStep())
		}
	}

	p.Length = length
}

// Clone creates a copy of the pattern
func (p *Pattern) Clone() *Pattern {
	newPattern := &Pattern{
		Steps:       make([]Step, len(p.Steps)),
		Length:      p.Length,
		Resolution:  p.Resolution,
		Name:        p.Name,
		SoundParams: make(map[string]float32),
	}

	for i, step := range p.Steps {
		newPattern.Steps[i] = step.Clone()
	}

	// Copy sound params
	for k, v := range p.SoundParams {
		newPattern.SoundParams[k] = v
	}

	return newPattern
}

// CountActiveSteps returns the number of active steps
func (p *Pattern) CountActiveSteps() int {
	count := 0
	for i := 0; i < p.Length; i++ {
		if p.Steps[i].Active {
			count++
		}
	}
	return count
}

// Shift shifts all steps by the specified amount (positive = right, negative = left)
// Wraps around at pattern boundaries
func (p *Pattern) Shift(amount int) {
	if amount == 0 || p.Length == 0 {
		return
	}

	// Normalize amount to pattern length
	amount = amount % p.Length
	if amount < 0 {
		amount += p.Length
	}

	// Create temporary copy
	temp := make([]Step, p.Length)
	for i := 0; i < p.Length; i++ {
		newIndex := (i + amount) % p.Length
		temp[newIndex] = p.Steps[i]
	}

	// Copy back
	for i := 0; i < p.Length; i++ {
		p.Steps[i] = temp[i]
	}
}

// Reverse reverses the pattern
func (p *Pattern) Reverse() {
	for i := 0; i < p.Length/2; i++ {
		j := p.Length - 1 - i
		p.Steps[i], p.Steps[j] = p.Steps[j], p.Steps[i]
	}
}

// Randomize randomizes the pattern
// density: 0.0 to 1.0 (0.0 = empty, 1.0 = all steps active)
func (p *Pattern) Randomize(density float32, randomFunc func() float32) {
	if density < 0.0 {
		density = 0.0
	}
	if density > 1.0 {
		density = 1.0
	}

	for i := 0; i < p.Length; i++ {
		if randomFunc() < density {
			p.Steps[i].Active = true
			// Random accent (30% chance)
			p.Steps[i].Accent = randomFunc() < 0.3
		} else {
			p.Steps[i].Active = false
		}
	}
}

// GenerateHiHat8ths generates a classic 8th note hi-hat pattern (every other 16th)
func (p *Pattern) GenerateHiHat8ths() {
	p.ClearAll()
	for i := 0; i < p.Length; i += 2 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.8
	}
}

// GenerateHiHat16ths generates a 16th note hi-hat pattern (all steps)
func (p *Pattern) GenerateHiHat16ths() {
	p.ClearAll()
	for i := 0; i < p.Length; i++ {
		p.Steps[i].Active = true
		// Alternate velocity for groove
		if i%2 == 0 {
			p.Steps[i].Velocity = 0.9
		} else {
			p.Steps[i].Velocity = 0.6
		}
	}
}

// GenerateHiHatSwing generates a swung hi-hat pattern
func (p *Pattern) GenerateHiHatSwing() {
	p.ClearAll()
	for i := 0; i < p.Length; i += 4 {
		// On-beat (strong)
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.9

		// Swung off-beat (weak, delayed feel)
		if i+3 < p.Length {
			p.Steps[i+3].Active = true
			p.Steps[i+3].Velocity = 0.5
		}
	}
}

// GenerateHiHatOffbeat generates offbeat hi-hats (backbeat feel)
func (p *Pattern) GenerateHiHatOffbeat() {
	p.ClearAll()
	for i := 1; i < p.Length; i += 2 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.8
	}
}

// GenerateHiHatSparse generates a sparse, minimal hi-hat pattern
func (p *Pattern) GenerateHiHatSparse() {
	p.ClearAll()
	for i := 0; i < p.Length; i += 4 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.75
	}
}

// GenerateDrumFill generates a drum fill pattern (increasing density toward end)
func (p *Pattern) GenerateDrumFill() {
	p.ClearAll()

	// For longer patterns (32/64), use EDM-style: build early, intense fill in last bar
	if p.Length > 16 {
		fillStart := p.Length - 16 // Last 16 steps for intense fill

		// Early section: sparse hits every 4 steps
		for i := 0; i < fillStart; i++ {
			if i%4 == 0 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.6 + (float32(i)/float32(fillStart))*0.1
			}
		}

		// Last 16 steps: dense fill with crescendo
		for i := fillStart; i < p.Length; i++ {
			localPos := i - fillStart
			if localPos%2 == 0 || localPos >= 12 {
				p.Steps[i].Active = true
				progress := float32(localPos) / 16.0
				p.Steps[i].Velocity = 0.7 + progress*0.3
				if localPos%4 == 0 {
					p.Steps[i].Accent = true
				}
			}
		}
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length; i++ {
			density := float32(i) / float32(p.Length)
			if i >= p.Length/2 {
				if i%2 == 0 || i >= p.Length*3/4 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.7 + density*0.3
					if i%4 == 0 {
						p.Steps[i].Accent = true
					}
				}
			} else {
				if i%4 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.7
				}
			}
		}
	}
}

// GenerateHiHatBuildup generates an accelerating hi-hat buildup
func (p *Pattern) GenerateHiHatBuildup() {
	p.ClearAll()

	// For longer patterns (32/64), use EDM-style: gradual build with intense last bar
	if p.Length > 16 {
		buildupEnd := p.Length - 16 // Last 16 steps for dense roll

		// Early section: slow quarter notes
		for i := 0; i < buildupEnd/2; i++ {
			if i%4 == 0 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.6
			}
		}

		// Middle section: eighth notes building
		for i := buildupEnd / 2; i < buildupEnd; i++ {
			if i%2 == 0 {
				p.Steps[i].Active = true
				progress := float32(i-buildupEnd/2) / float32(buildupEnd/2)
				p.Steps[i].Velocity = 0.65 + progress*0.1
			}
		}

		// Last 16 steps: dense 16th note roll with crescendo
		for i := buildupEnd; i < p.Length; i++ {
			p.Steps[i].Active = true
			progress := float32(i-buildupEnd) / 16.0
			p.Steps[i].Velocity = 0.7 + progress*0.3
		}
	} else {
		// Original 16-step behavior: 4ths → 8ths → 16ths
		for i := 0; i < p.Length; i++ {
			if i < p.Length/4 {
				if i%4 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.6
				}
			} else if i < p.Length/2 {
				if i%2 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.7
				}
			} else {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.7 + (float32(i-p.Length/2)/float32(p.Length/2))*0.3
			}
		}
	}
}

// GenerateSnareRoll generates a snare roll pattern
func (p *Pattern) GenerateSnareRoll() {
	p.ClearAll()
	// 16th note roll, increasing in velocity
	for i := 0; i < p.Length; i++ {
		p.Steps[i].Active = true
		// Crescendo
		progress := float32(i) / float32(p.Length)
		p.Steps[i].Velocity = 0.5 + progress*0.5
		// Accent every 4th for groove
		if i%4 == 0 {
			p.Steps[i].Accent = true
		}
	}
}

// GenerateKickPattern4OnFloor generates a classic four-on-the-floor kick pattern
func (p *Pattern) GenerateKickPattern4OnFloor() {
	p.ClearAll()
	for i := 0; i < p.Length; i += 4 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.9
		p.Steps[i].Accent = true
	}
}

// GenerateDrumFillRoll generates a dense 16th note roll with crescendo
func (p *Pattern) GenerateDrumFillRoll() {
	p.ClearAll()

	// For longer patterns, sparse build then dense roll in last bar
	if p.Length > 16 {
		rollStart := p.Length - 16

		// Early section: occasional hits to set up the roll
		for i := 0; i < rollStart; i++ {
			if i%8 == 0 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.6
			}
		}

		// Last 16 steps: full 16th note roll with crescendo
		for i := rollStart; i < p.Length; i++ {
			p.Steps[i].Active = true
			progress := float32(i-rollStart) / 16.0
			p.Steps[i].Velocity = 0.6 + progress*0.4
			if (i-rollStart)%4 == 0 {
				p.Steps[i].Accent = true
			}
		}
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length; i++ {
			p.Steps[i].Active = true
			progress := float32(i) / float32(p.Length)
			p.Steps[i].Velocity = 0.6 + progress*0.4
			if i%4 == 0 {
				p.Steps[i].Accent = true
			}
		}
	}
}

// GenerateDrumFillTriplet generates a triplet-feel fill
func (p *Pattern) GenerateDrumFillTriplet() {
	p.ClearAll()

	// For longer patterns, sparse triplets then dense in last bar
	if p.Length > 16 {
		tripletStart := p.Length - 16

		// Early section: occasional triplet groups
		for i := 0; i < tripletStart; i++ {
			if i%6 == 0 || i%6 == 3 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.65
			}
		}

		// Last 16 steps: full triplet feel with crescendo
		for i := tripletStart; i < p.Length; i++ {
			if (i-tripletStart)%3 == 0 {
				p.Steps[i].Active = true
				progress := float32(i-tripletStart) / 16.0
				p.Steps[i].Velocity = 0.7 + progress*0.3
				if (i-tripletStart)%12 == 0 {
					p.Steps[i].Accent = true
				}
			}
		}
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length; i++ {
			if i%3 == 0 {
				p.Steps[i].Active = true
				progress := float32(i) / float32(p.Length)
				p.Steps[i].Velocity = 0.7 + progress*0.3
				if i%12 == 0 {
					p.Steps[i].Accent = true
				}
			}
		}
	}
}

// GenerateDrumFillSparse generates sparse accent hits building to climax
func (p *Pattern) GenerateDrumFillSparse() {
	p.ClearAll()

	// For longer patterns, very sparse early, denser in last bar
	if p.Length > 16 {
		sparseEnd := p.Length - 16

		// Early section: very sparse (every 8 steps)
		for i := 0; i < sparseEnd; i += 8 {
			p.Steps[i].Active = true
			p.Steps[i].Velocity = 0.7
			p.Steps[i].Accent = true
		}

		// Last 16 steps: increasing density with accents
		for i := sparseEnd; i < p.Length; i += 2 {
			localPos := i - sparseEnd
			if localPos >= 8 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.8
				p.Steps[i].Accent = true
			}
		}

		// Final massive hit
		p.Steps[p.Length-1].Active = true
		p.Steps[p.Length-1].Velocity = 1.0
		p.Steps[p.Length-1].Accent = true
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length; i += 2 {
			if i >= p.Length/2 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.8
				p.Steps[i].Accent = true
			}
		}
		if p.Length > 0 {
			p.Steps[p.Length-1].Active = true
			p.Steps[p.Length-1].Velocity = 1.0
			p.Steps[p.Length-1].Accent = true
		}
	}
}

// GenerateHiHatBuildupRoll generates a fast roll at the end
func (p *Pattern) GenerateHiHatBuildupRoll() {
	p.ClearAll()

	// For longer patterns, gradual build with intense roll in last bar
	if p.Length > 16 {
		rollStart := p.Length - 16

		// Early section: very slow (whole notes)
		for i := 0; i < rollStart/3; i++ {
			if i%8 == 0 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.55
			}
		}

		// Middle section: quarter notes building to eighths
		for i := rollStart / 3; i < rollStart; i++ {
			if i < rollStart*2/3 {
				if i%4 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.6
				}
			} else {
				if i%2 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.65
				}
			}
		}

		// Last 16 steps: full 16th note roll with massive crescendo
		for i := rollStart; i < p.Length; i++ {
			p.Steps[i].Active = true
			progress := float32(i-rollStart) / 16.0
			p.Steps[i].Velocity = 0.7 + progress*0.3
		}
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length*3/4; i++ {
			if i < p.Length/4 {
				if i%4 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.6
				}
			} else {
				if i%2 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.7
				}
			}
		}
		for i := p.Length * 3 / 4; i < p.Length; i++ {
			p.Steps[i].Active = true
			progress := float32(i-p.Length*3/4) / float32(p.Length/4)
			p.Steps[i].Velocity = 0.7 + progress*0.3
		}
	}
}

// GenerateHiHatBuildupStutter generates a stutter pattern buildup
func (p *Pattern) GenerateHiHatBuildupStutter() {
	p.ClearAll()

	// For longer patterns, simple pattern early, intense stutter in last bar
	if p.Length > 16 {
		stutterStart := p.Length - 16

		// Early section: simple quarter notes
		for i := 0; i < stutterStart; i++ {
			if i%4 == 0 {
				p.Steps[i].Active = true
				p.Steps[i].Velocity = 0.6
			}
		}

		// Last 16 steps: intense stutter pattern with crescendo
		for i := stutterStart; i < p.Length; i++ {
			localPos := i - stutterStart
			// Stutter: play 2, skip 1, play 1 (creates rhythmic tension)
			mod := localPos % 4
			if mod == 0 || mod == 1 || mod == 3 {
				p.Steps[i].Active = true
				progress := float32(localPos) / 16.0
				p.Steps[i].Velocity = 0.6 + progress*0.4
			}
		}
	} else {
		// Original 16-step behavior
		for i := 0; i < p.Length; i++ {
			if i < p.Length/2 {
				if i%4 == 0 {
					p.Steps[i].Active = true
					p.Steps[i].Velocity = 0.6
				}
			} else {
				mod := (i - p.Length/2) % 4
				if mod == 0 || mod == 1 || mod == 3 {
					p.Steps[i].Active = true
					progress := float32(i-p.Length/2) / float32(p.Length/2)
					p.Steps[i].Velocity = 0.6 + progress*0.4
				}
			}
		}
	}
}

// ============================================================================
// KICK PATTERN VARIATIONS
// ============================================================================

// GenerateKickOffbeat generates kicks on the offbeats (techno/house variation)
func (p *Pattern) GenerateKickOffbeat() {
	p.ClearAll()
	for i := 2; i < p.Length; i += 4 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.9
		p.Steps[i].Accent = true
	}
}

// GenerateKickSyncopated generates a syncopated kick pattern
func (p *Pattern) GenerateKickSyncopated() {
	p.ClearAll()
	// Pattern: X . . x . . X . . x . x X . . .
	pattern := []int{0, 3, 6, 9, 11, 12}
	for _, step := range pattern {
		if step < p.Length {
			p.Steps[step].Active = true
			p.Steps[step].Velocity = 0.85
			if step%4 == 0 {
				p.Steps[step].Accent = true
			}
		}
	}
	// Repeat pattern if length > 16
	if p.Length > 16 {
		for _, step := range pattern {
			pos := step + 16
			if pos < p.Length {
				p.Steps[pos].Active = true
				p.Steps[pos].Velocity = 0.85
				if step%4 == 0 {
					p.Steps[pos].Accent = true
				}
			}
		}
	}
}

// GenerateKickBroken generates a broken beat kick pattern
func (p *Pattern) GenerateKickBroken() {
	p.ClearAll()
	// Broken beat: emphasis on 1, skip 2, hit on & of 3, skip 4
	for bar := 0; bar < p.Length/16; bar++ {
		offset := bar * 16
		if offset < p.Length {
			p.Steps[offset].Active = true // Beat 1
			p.Steps[offset].Velocity = 0.95
			p.Steps[offset].Accent = true
		}
		if offset+7 < p.Length {
			p.Steps[offset+7].Active = true // & of 2
			p.Steps[offset+7].Velocity = 0.8
		}
		if offset+10 < p.Length {
			p.Steps[offset+10].Active = true // & of 3
			p.Steps[offset+10].Velocity = 0.85
		}
		if offset+14 < p.Length {
			p.Steps[offset+14].Active = true // & of 4
			p.Steps[offset+14].Velocity = 0.75
		}
	}
}

// ============================================================================
// SNARE PATTERN VARIATIONS
// ============================================================================

// GenerateSnareBackbeat generates classic backbeat (2 and 4)
func (p *Pattern) GenerateSnareBackbeat() {
	p.ClearAll()
	for i := 4; i < p.Length; i += 8 {
		p.Steps[i].Active = true
		p.Steps[i].Velocity = 0.9
		p.Steps[i].Accent = true
	}
}

// GenerateSnareClap generates clap-style rhythm pattern
func (p *Pattern) GenerateSnareClap() {
	p.ClearAll()
	// Double claps on 2 and 4
	for bar := 0; bar < p.Length/16; bar++ {
		offset := bar * 16
		// Beat 2 with double
		if offset+4 < p.Length {
			p.Steps[offset+4].Active = true
			p.Steps[offset+4].Velocity = 0.85
			p.Steps[offset+4].Accent = true
		}
		if offset+5 < p.Length {
			p.Steps[offset+5].Active = true
			p.Steps[offset+5].Velocity = 0.75
		}
		// Beat 4 with double
		if offset+12 < p.Length {
			p.Steps[offset+12].Active = true
			p.Steps[offset+12].Velocity = 0.85
			p.Steps[offset+12].Accent = true
		}
		if offset+13 < p.Length {
			p.Steps[offset+13].Active = true
			p.Steps[offset+13].Velocity = 0.75
		}
	}
}

// GenerateSnareSyncopated generates syncopated snare hits
func (p *Pattern) GenerateSnareSyncopated() {
	p.ClearAll()
	// Pattern: . . . x . . x . . x . . x x . .
	pattern := []int{3, 6, 9, 12, 13}
	for _, step := range pattern {
		if step < p.Length {
			p.Steps[step].Active = true
			p.Steps[step].Velocity = 0.8
			if step == 12 || step == 13 {
				p.Steps[step].Accent = true
			}
		}
	}
	// Repeat for longer patterns
	if p.Length > 16 {
		for _, step := range pattern {
			pos := step + 16
			if pos < p.Length {
				p.Steps[pos].Active = true
				p.Steps[pos].Velocity = 0.8
				if step == 12 || step == 13 {
					p.Steps[pos].Accent = true
				}
			}
		}
	}
}

// ============================================================================
// HI-HAT PATTERN VARIATIONS (Additional)
// ============================================================================

// GenerateHiHatTriplet generates triplet hi-hat groove
func (p *Pattern) GenerateHiHatTriplet() {
	p.ClearAll()
	for i := 0; i < p.Length; i++ {
		if i%3 == 0 {
			p.Steps[i].Active = true
			p.Steps[i].Velocity = 0.7
			if i%12 == 0 {
				p.Steps[i].Accent = true
			}
		}
	}
}

// ApplyEuclidean generates a Euclidean rhythm pattern
// pulses = number of hits, rotation = steps to rotate pattern
// Does not clear existing pattern - allows layering
func (p *Pattern) ApplyEuclidean(pulses, rotation int, velocity float32) {
	indices := GenerateEuclidean(pulses, p.Length)

	// Apply rotation
	for _, idx := range indices {
		rotatedIdx := (idx + rotation) % p.Length
		p.Steps[rotatedIdx].Active = true
		p.Steps[rotatedIdx].Velocity = velocity
	}
}

// ApplyEuclideanClear generates a Euclidean rhythm pattern (clearing existing)
func (p *Pattern) ApplyEuclideanClear(pulses, rotation int, velocity float32) {
	p.ClearAll()
	p.ApplyEuclidean(pulses, rotation, velocity)
}

// RotateLeft shifts all steps left by 1 position (first step wraps to end)
// This is useful for nudging patterns to find grooves
func (p *Pattern) RotateLeft() {
	if p.Length <= 1 {
		return
	}

	// Save first step
	firstStep := p.Steps[0]

	// Shift all steps left
	for i := 0; i < p.Length-1; i++ {
		p.Steps[i] = p.Steps[i+1]
	}

	// Wrap first step to end
	p.Steps[p.Length-1] = firstStep
}

// RotateRight shifts all steps right by 1 position (last step wraps to beginning)
// This is useful for nudging patterns to find grooves
func (p *Pattern) RotateRight() {
	if p.Length <= 1 {
		return
	}

	// Save last step
	lastStep := p.Steps[p.Length-1]

	// Shift all steps right
	for i := p.Length - 1; i > 0; i-- {
		p.Steps[i] = p.Steps[i-1]
	}

	// Wrap last step to beginning
	p.Steps[0] = lastStep
}

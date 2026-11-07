package groove

import (
	"fmt"
	"math/rand/v2"
	"reflect"
)

type PerformanceKind int

const (
	NoPerformance PerformanceKind = iota
	DrumFill
	TrackVariation
	GrooveVariation
)

func (k PerformanceKind) Name() string {
	switch k {
	case DrumFill:
		return "Drum fill"
	case TrackVariation:
		return "Track variation"
	case GrooveVariation:
		return "Groove variation"
	}
	return ""
}

// Immutable prepared patterns cross to Render through an atomic pointer.
// No project slots or project history are changed by a performance gesture.
type performanceTake struct {
	generation uint64
	kind       PerformanceKind
	source     int
	patterns   map[string]Pattern
}

func (p *Player) QueuePerformance(project Project, kind PerformanceKind, trackID string) error {
	if !p.Playing() {
		return fmt.Errorf("start playback before queuing a performance")
	}
	if kind < DrumFill || kind > GrooveVariation {
		return fmt.Errorf("unknown performance action")
	}
	if kind == TrackVariation && project.Track(trackID) == nil {
		return fmt.Errorf("select a track to vary")
	}
	source := p.CurrentPattern()
	if source < 0 || source >= PatternCount {
		source = project.Pattern
	}
	take := &performanceTake{generation: p.performanceGeneration.Load(), kind: kind, source: source, patterns: make(map[string]Pattern, len(project.Tracks))}
	affected := false
	for _, track := range project.Tracks {
		pattern := clonePattern(track.Patterns[source])
		switch kind {
		case DrumFill:
			if track.Kind == Sampler || track.Kind == Acid || track.Kind == Dual || track.Kind == Crash {
				break
			}
			affected = true
			if len(pattern.Steps) < pattern.Resolution {
				original := clonePattern(pattern)
				pattern.Steps = make([]Step, pattern.Resolution)
				for i := range pattern.Steps {
					step := original.Steps[i%len(original.Steps)]
					step.Locks = cloneMap(step.Locks)
					pattern.Steps[i] = step
				}
			}
			beat := max(1, pattern.Resolution/4)
			for i := range pattern.Steps {
				if i%pattern.Resolution < pattern.Resolution-beat {
					continue
				}
				step := pattern.Steps[i]
				step.Active = true
				step.Probability = 1
				step.Every = 0
				step.Velocity = .55 + .35*float64(i%beat+1)/float64(beat)
				step.Accent = i%beat == beat-1
				if track.Kind == Tom {
					step.Locks = cloneMap(step.Locks)
					if step.Locks == nil {
						step.Locks = map[string]float64{}
					}
					step.Locks["pitch"] = max(40, min(300, track.Parameters["pitch"]*(1.5-.7*float64(i%beat)/float64(max(1, beat-1)))))
				}
				pattern.Steps[i] = step
			}
		case TrackVariation, GrooveVariation:
			if kind == TrackVariation && track.ID != trackID {
				break
			}
			affected = true
			scratch := track
			scratch.Patterns = []Pattern{pattern}
			Generate(&scratch, 0, "Mutate")
			audible := min(len(pattern.Steps), pattern.Resolution)
			if reflect.DeepEqual(pattern.Steps[:audible], track.Patterns[source].Steps[:audible]) {
				i := rand.IntN(audible)
				pattern.Steps[i].Active = !pattern.Steps[i].Active
			}
		}
		take.patterns[track.ID] = pattern
	}
	if !affected {
		return fmt.Errorf("add a drum track for a fill, or a track to vary")
	}
	p.performancePending.Store(take)
	return nil
}
func (p *Player) ClearPerformanceTake() {
	p.performanceGeneration.Add(1)
	p.CancelPerformance()
	p.performanceLast.Store(nil)
}

func (p *Player) CancelPerformance() { p.performancePending.Store(nil) }
func (p *Player) QueuedPerformance() PerformanceKind {
	if take := p.performancePending.Load(); p.Playing() && take != nil && take.generation == p.performanceGeneration.Load() {
		return take.kind
	}
	return NoPerformance
}
func (p *Player) RunningPerformance() PerformanceKind {
	if take := p.performanceVisible.Load(); p.Playing() && take != nil && take.generation == p.performanceGeneration.Load() {
		return take.kind
	}
	return NoPerformance
}
func (p *Player) HasPerformanceTake() bool {
	take := p.performanceLast.Load()
	return take != nil && take.generation == p.performanceGeneration.Load()
}

// Keep the most recently launched take in an unused scene, with normal project
// undo. Drop locks for effect units removed since the take was prepared.
func (p *Player) KeepPerformance(project *Project, slot int) error {
	take := p.performanceLast.Load()
	if take == nil || take.generation != p.performanceGeneration.Load() {
		return fmt.Errorf("play a fill or variation first")
	}
	if slot < 0 || slot >= PatternCount {
		return fmt.Errorf("no empty pattern slot available")
	}
	for _, track := range project.Tracks {
		if track.Patterns[slot].State != nil {
			return fmt.Errorf("choose an empty pattern slot")
		}
		for _, step := range track.Patterns[slot].Steps {
			if step.Active || len(step.Locks) > 0 {
				return fmt.Errorf("choose an empty pattern slot")
			}
		}
	}
	matched := false
	for _, track := range project.Tracks {
		if _, ok := take.patterns[track.ID]; ok {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("the last take belongs to a previous project")
	}
	for i := range project.Tracks {
		track := &project.Tracks[i]
		if pattern, ok := take.patterns[track.ID]; ok {
			pattern = clonePattern(pattern)
			allowed := map[string]bool{}
			for _, param := range LockParameters(withTrackState(*track, pattern.State)) {
				allowed[param.ID] = true
			}
			for i := range pattern.Steps {
				for id := range pattern.Steps[i].Locks {
					if !allowed[id] {
						delete(pattern.Steps[i].Locks, id)
					}
				}
			}
			track.Patterns[slot] = pattern
		}
	}
	return nil
}

func (p *Player) performancePattern(t *playTrack) *Pattern {
	if p.performance != nil {
		// Take values are immutable. Render only reads this copy.
		if pattern, ok := p.performance.patterns[t.track.ID]; ok {
			t.performancePattern = pattern
			return &t.performancePattern
		}
	}
	return &t.track.Patterns[p.renderPattern]
}
func (p *Player) advancePerformance() {
	previous := p.performance
	p.performance = p.performancePending.Swap(nil)
	if p.performance != nil && p.performance.generation != p.performanceGeneration.Load() {
		p.performance = nil
	}
	p.performanceVisible.Store(p.performance)
	if p.performance != nil {
		p.performanceLast.Store(p.performance)
	}
	if previous != nil || p.performance != nil {
		for i := range p.active.tracks {
			t := &p.active.tracks[i]
			t.voice.reset()
			t.voice.modulation(t.track, p.active.project.BPM, p.performancePattern(t).Resolution)
		}
	}
}

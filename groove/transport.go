package groove

func (p *Player) CurrentPattern() int {
	if p.device != nil && p.Playing() {
		frame := p.audibleFrame()
		for _, voice := range p.cache {
			if pattern := voice.patternAt(frame); pattern >= 0 {
				return pattern
			}
		}
	}
	return int(p.currentPattern.Load())
}
func (p *Player) QueuedPattern() int  { return int(p.queuedPattern.Load()) }
func (p *Player) SongSection() int    { return int(p.currentSection.Load()) }
func (p *Player) SongBar() int        { return int(p.currentBar.Load()) }
func (p *Player) SongMode() bool      { return p.songMode.Load() }
func (p *Player) SetSongMode(on bool) { p.songMode.Store(on) }

// LaunchedPattern follows the render transport, including repeated launches of
// the same song pattern. Control surfaces use it to recall their matching knobs.
func (p *Player) LaunchedPattern() (int, uint64) {
	launch := p.patternLaunch.Load()
	return int(p.currentPattern.Load()), launch
}

// BarClock is how far the render has played since Play, in 4/4 bars. It runs
// ahead of what is audible by the output buffer, which is what an edit meant
// for the next bar needs.
func (p *Player) BarClock() float64 {
	v := p.barClock.Load()
	return float64(v>>16) + float64(v&65535)/65536
}

func (p *Player) beginTransport(song bool) {
	p.barsPlayed = 0
	p.barClock.Store(0)
	p.performance = nil
	p.performanceVisible.Store(nil)
	p.barPhase = 0
	p.sectionIndex, p.sectionBars = 0, 0
	p.playingSong = song && len(p.active.project.Song) > 0
	p.requestedPattern = p.active.project.Pattern
	pattern := p.requestedPattern
	if p.playingSong {
		pattern = p.active.project.Song[0].Pattern
	}
	p.sectionMutes = nil
	if p.playingSong {
		p.sectionMutes = p.active.project.Song[0].Mutes
	}
	p.switchPattern(pattern)
	p.applySectionMix()
	p.publishTransport()
}
func (p *Player) switchPattern(pattern int) {
	previous := p.active.tracks
	p.renderPattern = pattern
	p.currentPattern.Store(int32(pattern))
	p.patternLaunch.Add(1)
	p.liveControls = !p.playingSong && pattern == p.active.project.Pattern
	p.active.tracks = p.active.banks[pattern]
	if p.liveControls {
		p.active.tracks = p.active.live
	}
	// Shared units keep their tails. Units returning after being absent start
	// fresh rather than replaying a delay/reverb buffer frozen several bars ago.
	for i := range p.active.tracks {
		for _, effect := range p.active.tracks[i].effects {
			shared := false
			for _, old := range previous {
				if old.track.ID != p.active.tracks[i].track.ID {
					continue
				}
				for _, active := range old.effects {
					shared = shared || active.state == effect.state
				}
			}
			if !shared {
				effect.state.reset()
			}
		}
	}
	// The full sound and sample change at the same boundary as the sequence.
	p.configureTracks(true, false)
}

func (p *Player) adoptTrackBank(previous *playback) {
	next := p.active
	if previous != nil && next.liveRevision != previous.liveRevision && next.project.Pattern == p.renderPattern {
		p.liveControls = true
	}
	next.tracks = next.banks[p.renderPattern]
	if p.liveControls && next.project.Pattern == p.renderPattern {
		next.tracks = next.live
	} else if !p.playingSong && p.Playing() && previous != nil && next.project.Pattern != p.renderPattern {
		// UI recall happens immediately; the sounding bank keeps its live
		// controls until the queued pattern launches. Carry only matching IDs.
		for i := range next.tracks {
			for _, old := range previous.tracks {
				if old.track.ID == next.tracks[i].track.ID && old.track.Kind == next.tracks[i].track.Kind {
					old.track.Patterns = next.tracks[i].track.Patterns
					next.tracks[i] = old
					break
				}
			}
		}
	}
}

func (p *Player) configureTracks(resetVoice, resetFX bool) {
	for i := range p.active.tracks {
		t := &p.active.tracks[i]
		if t.voice.sample != t.track.Sample {
			t.voice.sample, t.voice.samplePlaying = t.track.Sample, false
		}
		if resetVoice {
			t.voice.reset()
		}
		for id, value := range t.track.Parameters {
			t.voice.parameter(id, value)
		}
		t.resolution = p.performancePattern(t).Resolution
		t.voice.modulation(t.track, p.active.project.BPM, t.resolution)
		if resetVoice {
			for _, lfo := range t.voice.lfos() {
				if lfo != nil {
					lfo.Reset()
				}
			}
		}
		t.effectParameters(p.active.project.BPM)
		if resetFX {
			for _, effect := range t.effects {
				effect.state.reset()
			}
		}
		index := int(t.voice.position.Load())
		pattern := p.performancePattern(t)
		if index >= 0 && index < len(pattern.Steps) {
			for id, value := range pattern.Steps[index].Locks {
				t.parameter(id, value, p.active.project.BPM)
			}
		}
	}
}
func (p *Player) publishTransport() {
	queued := -1
	if !p.songMode.Load() && p.requestedPattern != p.renderPattern {
		queued = p.requestedPattern
	}
	p.queuedPattern.Store(int32(queued))
	section := -1
	if p.playingSong {
		section = p.sectionIndex
	}
	p.currentSection.Store(int32(section))
	p.currentBar.Store(int32(p.sectionBars + 1))
}
func (p *Player) adoptTransport(reset bool) {
	if reset {
		p.beginTransport(false)
		return
	}
	if p.requestedPattern != p.active.project.Pattern && !p.songMode.Load() {
		p.CancelPerformance()
	}
	p.requestedPattern = p.active.project.Pattern
	if !p.Playing() && p.renderPattern != p.requestedPattern {
		p.switchPattern(p.requestedPattern)
	}
	if p.playingSong && p.sectionIndex < len(p.active.project.Song) {
		p.sectionMutes = p.active.project.Song[p.sectionIndex].Mutes
	}
	if !p.Playing() {
		p.sectionMutes = nil
	}
	p.applySectionMix()
	p.publishTransport()
}
func (p *Player) advanceBar() {
	p.barsPlayed++
	song := p.active.project.Song
	if p.songMode.Load() && len(song) > 0 {
		if !p.playingSong || p.sectionIndex >= len(song) {
			p.playingSong = true
			p.sectionIndex, p.sectionBars = 0, 0
			p.switchPattern(song[0].Pattern)
		} else {
			p.sectionBars++
			if p.sectionBars >= song[p.sectionIndex].Repeats {
				p.sectionBars = 0
				p.sectionIndex = (p.sectionIndex + 1) % len(song)
				p.switchPattern(song[p.sectionIndex].Pattern)
			} else if song[p.sectionIndex].Pattern != p.renderPattern {
				p.switchPattern(song[p.sectionIndex].Pattern)
			}
		}
	} else {
		p.playingSong = false
		p.sectionBars = 0
		if p.renderPattern != p.requestedPattern {
			p.switchPattern(p.requestedPattern)
		}
		if len(song) == 0 {
			p.songMode.Store(false)
		}
	}
	p.sectionMutes = nil
	if p.playingSong {
		p.sectionMutes = song[p.sectionIndex].Mutes
	}
	p.applySectionMix()
	p.advancePerformance()
	p.publishTransport()
}

// Preserve the sounding section's identity when its row is reordered. If it
// disappears, begin the updated song at the next bar rather than indexing stale rows.
func (p *Player) remapSong(next Project) {
	if !p.playingSong || p.active == nil || p.sectionIndex >= len(p.active.project.Song) {
		return
	}
	id := p.active.project.Song[p.sectionIndex].ID
	for i, section := range next.Song {
		if section.ID == id {
			p.sectionIndex = i
			return
		}
	}
	p.playingSong = false
}

func (p *Player) applySectionMix() {
	for i := range p.active.tracks {
		t := &p.active.tracks[i]
		muted := t.track.Mute
		if override, ok := p.sectionMutes[t.track.ID]; ok {
			muted = override
		}
		t.left, t.right = t.baseLeft, t.baseRight
		if muted || t.soloMuted {
			t.left, t.right = 0, 0
		}
	}
}

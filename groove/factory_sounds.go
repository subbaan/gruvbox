package groove

// FactorySounds returns fresh editable recipes, never shared mutable maps.
// For the 303 they double as the sound bank that Randomise sound and the
// groove styles draw from.
func FactorySounds(kind Kind) []SoundPreset {
	var presets []SoundPreset
	add := func(name, description string, values map[string]float64, fx ...Effect) {
		t := NewTrack(kind, 0)
		for key, value := range values {
			t.Parameters[key] = value
		}
		t.Effects = fx
		p := CaptureSound(t, name)
		p.Description = description
		presets = append(presets, p)
	}
	effect := func(kind EffectKind, values map[string]float64) Effect {
		e := NewEffect(kind)
		for key, value := range values {
			e.Parameters[key] = value
		}
		return e
	}
	switch kind {
	case Dual:
		add("Rounded mono", "A warm single-saw/sub bass. Short filter envelope, clean body and a firm gate.", map[string]float64{"cutoff": .32, "sub": .35, "amp_attack": .003, "amp_decay": .2, "amp_sustain": .5, "filter_decay": .18})
		add("Pulse pluck", "A hollow pulse with short envelopes. Try a busy bassline and a little PWM.", map[string]float64{"osc1_wave": 1, "pulse": .35, "amp_attack": .001, "amp_decay": .18, "amp_sustain": 0, "amp_release": .08, "filter_decay": .15, "resonance": .45})
		add("Detuned lead", "Two saws with a slight beating detune and a driven resonant filter.", map[string]float64{"osc2_wave": 0, "osc2_level": .6, "detune": 12, "cutoff": .55, "resonance": .4, "drive": .3, "amp_sustain": .7, "env_amount": .35})
		add("Dub interval", "Oscillator 2 a fifth above, with short envelopes, delay and a soft room tail.", map[string]float64{"osc2_level": .35, "osc2_tune": 7, "amp_sustain": 0, "amp_decay": .35, "amp_release": .35, "env_amount": .4}, effect(Delay, map[string]float64{"mix": .2}), effect(Reverb, map[string]float64{"mix": .15}))
		add("Slow pad", "Triangle and saw with slow envelopes and chorus. Extend notes in the piano roll to let the sound bloom.", map[string]float64{"osc1_wave": 2, "osc2_wave": 0, "osc2_level": .4, "sub": .1, "cutoff": .6, "amp_attack": .6, "amp_decay": .7, "amp_sustain": .8, "amp_release": 1.2, "filter_attack": 1, "filter_decay": 1, "filter_sustain": .6, "filter_release": 1.2}, effect(Chorus, map[string]float64{"mix": .25}))
		presets[len(presets)-1].LFOs[0] = LFO{Enabled: true, Target: 1, Rate: .2, Depth: .12, Sync: 1, Division: 4, Steps: 64}
		add("Rubber bass", "Pulse and triangle, pronounced envelope movement and longer legato glide. Add slide flags between notes.", map[string]float64{"osc1_wave": 1, "osc2_wave": 2, "osc2_level": .2, "cutoff": .35, "resonance": .6, "env_amount": .6, "filter_decay": .2, "glide": .12, "drive": .2})
		// alpha.62: sounds only two oscillators can make (sync, FM, ring mod,
		// unison and intervals), so the Dual has ground the SH-101 lacks.
		add("Sync lead", "Oscillator 2 synced to 1, its pitch swept by the filter envelope: the tearing lead. Hold notes and slide between them.", map[string]float64{"sync": 1, "osc1_level": .25, "osc2_level": 1, "osc2_tune": 7, "osc2_env": .4, "cutoff": .5, "resonance": .2, "env_amount": .3, "filter_attack": .002, "filter_decay": .45, "filter_sustain": .15, "amp_sustain": .8, "amp_release": .2, "glide": .08}, effect(Delay, map[string]float64{"division": .1875, "feedback": .35, "mix": .18}))
		add("Sync sweep", "Synced oscillator 2 swept by a one-bar LFO, so long notes change colour as they ring.", map[string]float64{"sync": 1, "osc1_level": .2, "osc2_level": 1, "osc2_tune": 5, "cutoff": .55, "resonance": .15, "env_amount": .2, "amp_sustain": .8, "amp_release": .25, "glide": .1}, effect(Chorus, map[string]float64{"mix": .2}))
		presets[len(presets)-1].LFOs[0] = LFO{Enabled: true, Target: 9, Waveform: 0, Rate: .5, Depth: .45, Sync: 1, Division: 1, Steps: 16}
		add("FM bell", "A sine carrier phase-modulated by a sine nineteen semitones up: a glassy bell with a long tail.", map[string]float64{"osc1_wave": 3, "osc2_wave": 3, "osc2_level": 0, "osc2_tune": 19, "fm": .45, "sub": 0, "cutoff": .85, "resonance": .1, "env_amount": .1, "amp_attack": .001, "amp_decay": .7, "amp_sustain": 0, "amp_release": .6}, effect(Reverb, map[string]float64{"room": .55, "mix": .18}))
		add("FM bass", "A sine carrier with an octave-up modulator: a round bass with a knocking attack.", map[string]float64{"osc1_wave": 3, "osc2_wave": 3, "osc2_level": 0, "osc2_tune": 12, "fm": .35, "sub": .3, "cutoff": .5, "resonance": .15, "env_amount": .35, "filter_decay": .12, "amp_decay": .25, "amp_sustain": .3})
		add("Ring clang", "Saw ring-modulated by a detuned pulse a fifth up: metallic, clangorous stabs.", map[string]float64{"osc2_wave": 1, "osc2_tune": 7, "detune": 13, "osc1_level": .2, "osc2_level": 0, "ring": .8, "sub": 0, "cutoff": .6, "resonance": .3, "env_amount": .4, "filter_decay": .15, "amp_decay": .2, "amp_sustain": 0, "amp_release": .15}, effect(Delay, map[string]float64{"division": .125, "feedback": .3, "mix": .18}))
		add("Unison lead", "Three detuned saws and an octave above: a wide lead for long, held notes.", map[string]float64{"unison": .6, "osc2_level": .4, "osc2_tune": 12, "detune": 4, "sub": .1, "cutoff": .5, "resonance": .25, "env_amount": .3, "filter_decay": .4, "filter_sustain": .3, "amp_sustain": .8, "amp_release": .3, "glide": .1}, effect(Chorus, map[string]float64{"mix": .2}), effect(Delay, map[string]float64{"division": .1875, "feedback": .35, "mix": .15}))
		add("Fifth stab", "Unison saws with oscillator 2 a fifth up: every note is a short two-note chord.", map[string]float64{"unison": .3, "osc2_level": .8, "osc2_tune": 7, "sub": .1, "cutoff": .4, "resonance": .35, "env_amount": .55, "filter_decay": .12, "amp_decay": .14, "amp_sustain": 0, "amp_release": .1}, effect(Delay, map[string]float64{"division": .1875, "feedback": .4, "mix": .2}))
		add("Octave stab", "Pulse and saw an octave apart with a quick filter blip, for offbeat stabs.", map[string]float64{"osc1_wave": 1, "pulse": .4, "osc2_level": .7, "osc2_tune": 12, "detune": 6, "cutoff": .42, "resonance": .4, "env_amount": .6, "filter_decay": .1, "amp_decay": .12, "amp_sustain": 0, "amp_release": .08}, effect(Chorus, map[string]float64{"mix": .2}))
		add("Minor stack", "Oscillator 2 a minor third up, unison and a soft attack: a dark chord on every note.", map[string]float64{"unison": .4, "osc2_level": .75, "osc2_tune": 3, "sub": .15, "cutoff": .38, "resonance": .25, "env_amount": .35, "filter_decay": .3, "amp_attack": .01, "amp_decay": .3, "amp_sustain": .2, "amp_release": .2}, effect(Reverb, map[string]float64{"room": .5, "mix": .15}))
	case SH101:
		add("101 bass", "Saw over a one-octave sub, a short filter envelope and a little resonance: the classic bass.", map[string]float64{"saw": .8, "sub": .6, "cutoff": .3, "resonance": .35, "env_amount": .55, "decay": .25, "sustain": .1, "release": .06})
		add("Sub pressure", "A square two octaves down under a quiet pulse. Deep and round; let notes ring.", map[string]float64{"saw": 0, "pulse": .4, "pw": 0, "sub": 1, "sub_mode": 1, "cutoff": .22, "resonance": .2, "env_amount": .35, "decay": .4, "sustain": .4, "release": .12})
		add("Acid 101", "Bright saw, high resonance and a snappy envelope. Accents open it up; slides glide.", map[string]float64{"saw": 1, "sub": .2, "cutoff": .25, "resonance": .78, "env_amount": .75, "decay": .18, "sustain": 0, "accent": .6, "glide": .06})
		add("Rubber pluck", "A hollow square with a quick filter blip, for busy bouncing lines.", map[string]float64{"saw": .2, "pulse": .8, "pw": .5, "cutoff": .35, "resonance": .5, "env_amount": .65, "decay": .14, "sustain": 0, "release": .1})
		add("PWM lead", "The LFO sweeps the pulse width for a moving, chorused tone; slides glide.", map[string]float64{"saw": 0, "pulse": 1, "pw": .7, "pwm_source": 1, "lfo_rate": .45, "sub": .2, "cutoff": .55, "resonance": .25, "env_amount": .3, "attack": .01, "decay": .6, "sustain": .7, "release": .25, "glide": .1}, effect(Delay, map[string]float64{"division": .1875, "feedback": .4, "mix": .2}))
		add("Squelch stab", "Saw and pulse through a resonant, fast envelope. Short chords of one note, with an echo.", map[string]float64{"saw": .7, "pulse": .5, "pw": .4, "sub": .1, "cutoff": .35, "resonance": .65, "env_amount": .8, "decay": .1, "sustain": 0}, effect(Delay, map[string]float64{"division": .125, "feedback": .3, "mix": .18}))
		add("Trill", "The LFO retriggers the envelope while a note is held: long notes stutter in time.", map[string]float64{"saw": .3, "pulse": .7, "pw": .3, "env_trigger": 2, "lfo_wave": 1, "lfo_rate": .62, "cutoff": .45, "resonance": .4, "env_amount": .5, "decay": .08, "sustain": 0}, effect(Delay, map[string]float64{"division": .1875, "feedback": .35, "mix": .2}))
		add("Siren", "The LFO bends the pitch; the gate holds the level. Long notes with a dub echo.", map[string]float64{"saw": .6, "pulse": .4, "pw": .2, "vco_lfo": .35, "lfo_rate": .35, "vca_mode": 1, "cutoff": .5, "resonance": .3, "env_amount": .2}, effect(Delay, map[string]float64{"division": .1875, "feedback": .6, "mix": .3}), effect(Reverb, map[string]float64{"room": .6, "mix": .15}))
		add("Noise zap", "Filtered noise only, tuned by key tracking: a percussive hit that follows the notes.", map[string]float64{"saw": 0, "sub": 0, "noise": 1, "cutoff": .5, "resonance": .7, "keytrack": 1, "env_amount": .5, "decay": .12, "sustain": 0})
		add("Hollow organ", "A square and its sub on the gate, no envelope movement: steady, hollow chords of one note.", map[string]float64{"saw": 0, "pulse": 1, "pw": 0, "sub": .5, "vca_mode": 1, "cutoff": .5, "resonance": .1, "env_amount": .1, "keytrack": .8}, effect(Chorus, map[string]float64{"mix": .25}))
		add("Sample and hold", "Random LFO steps on the filter: every note blips differently.", map[string]float64{"saw": .8, "sub": .3, "lfo_wave": 2, "lfo_rate": .55, "filter_lfo": .4, "cutoff": .35, "resonance": .6, "env_amount": .3, "decay": .2, "sustain": .2})
	case Acid:
		add("Rounded sub", "Low cutoff, gentle resonance and a strong sub. Try short notes with occasional slides.", map[string]float64{"cutoff": .3, "resonance": .25, "env": .35, "decay": .35, "sub": .5})
		add("Dry square", "Hollow square-wave bass with a short envelope, ready for a busy pattern.", map[string]float64{"square": 1, "cutoff": .45, "resonance": .4, "env": .65, "decay": .2, "sub": .15})
		add("Resonant acid", "Bright saw, pronounced resonance, light drive and a tempo-synced eighth-note delay.", map[string]float64{"cutoff": .65, "resonance": .82, "env": .8, "decay": .4}, effect(Distortion, map[string]float64{"drive": 3, "mix": .2}), effect(Delay, map[string]float64{"division": .125, "feedback": .28, "mix": .18}))
		add("Slow motion", "A four-bar cutoff LFO and light stereo chorus. Start with a sparse bassline.", map[string]float64{"cutoff": .5, "resonance": .55, "decay": .6}, effect(Chorus, map[string]float64{"rate": .3, "depth": .4, "mix": .2}))
		presets[len(presets)-1].LFOs[0] = LFO{Enabled: true, Target: 1, Rate: .2, Depth: .15, Sync: 1, Division: 4, Steps: 64}
		acidBank(add, effect, func(l LFO) { presets[len(presets)-1].LFOs[0] = l })
	case Kick:
		add("Tight punch", "Short, clean kick with a defined click and quick pitch fall.", map[string]float64{"decay": .2, "drive": .2, "click": .2, "pitch_decay": .012})
		add("Deep thump", "Lower tuning and a longer body, with a softer transient.", map[string]float64{"tune": -4, "decay": .5, "drive": .3, "click": .05, "pitch_decay": .03})
		add("Driven floor", "Firm low end and a dirty edge for four-on-the-floor patterns.", map[string]float64{"decay": .35, "drive": .7, "click": .15, "pitch_decay": .02})
	case Snare:
		add("Dry snap", "Short noise tail and a clear tonal body.", map[string]float64{"snap": .8, "tone": .45, "decay": .1, "tone_decay": .055})
		add("Dusty backbeat", "Lower snare with a soft noise tail and a touch of bit crushing.", map[string]float64{"tune": -3, "snap": .5, "tone": .55, "decay": .22}, effect(BitCrusher, map[string]float64{"bits": 10, "rate": 2, "mix": .18}))
		add("Wire rattle", "A bright, airy noise tail with very little tonal body.", map[string]float64{"tune": 2, "snap": .9, "tone": .15, "decay": .25, "tone_decay": .035})
	case ClosedHat:
		add("Tight tick", "A short metallic hat for fast rhythms.", map[string]float64{"decay": .025, "tone": .85, "noise": .25})
		add("Soft shuffle", "A slightly longer, noisier hat for sparse or swung patterns.", map[string]float64{"decay": .09, "tone": .45, "noise": .7})
	case OpenHat:
		add("Bright lift", "Bright metallic sustain that opens up a groove.", map[string]float64{"decay": .3, "tone": .85, "noise": .3})
		add("Loose wash", "Longer, softer and noisier, with a small room around it.", map[string]float64{"decay": .5, "tone": .4, "noise": .75}, effect(Reverb, map[string]float64{"room": .3, "mix": .12}))
	case Clap:
		add("Dry hands", "Short clap, little drive and a sharp upper tone.", map[string]float64{"decay": .05, "tone": .75, "drive": .1})
		add("Warehouse clap", "Lower, fuller clap with a moderate room tail.", map[string]float64{"tune": -2, "decay": .12, "tone": .5, "drive": .4}, effect(Reverb, map[string]float64{"room": .65, "damping": .6, "mix": .2}))
	case Rimshot:
		add("Wood knock", "Lower pitch and a rounded tonal hit.", map[string]float64{"pitch": 350, "decay": .055, "tone": .85, "snap": .4})
		add("Sharp rim", "A high, short, snappy accent.", map[string]float64{"pitch": 650, "decay": .025, "tone": .4, "snap": .9})
	case Tom:
		add("Bass tone", "C2 root, no pitch sweep or attack noise. Use piano-roll notes for a bassline.", map[string]float64{"pitch": 65.40639132514966, "pitch_decay": 0, "tone": 1, "attack": 0, "decay": .4})
		add("Low tom", "A low, rounded drum with a subtle pitch drop.", map[string]float64{"pitch": 80, "pitch_decay": .3, "decay": .55, "tone": .9, "attack": .2})
		add("Tiny conga", "High root pitch, a short body and a light attack. Piano-roll notes vary the pitch.", map[string]float64{"pitch": 220, "pitch_decay": .15, "decay": .18, "tone": .85, "attack": .25})
		add("Pitched echoes", "A clean pitched tom with dotted-eighth echoes. Notes transpose from the C2 root.", map[string]float64{"pitch": 65.40639132514966, "pitch_decay": 0, "tone": 1, "attack": 0, "decay": .3}, effect(Delay, map[string]float64{"division": .1875, "feedback": .32, "mix": .22}))
	case Crash:
		add("Short splash", "A quick, bright cymbal accent.", map[string]float64{"decay": .6, "tone": .8, "metallic": .85})
		add("Dark crash", "A longer cymbal wash with less top end.", map[string]float64{"decay": 2, "tone": .35, "metallic": .5})
	case Sampler:
		// No blank factory recipe: recalling a sampler sound includes its WAV.
	}
	drumBank(kind, add, effect)
	return presets
}

// FactorySoundNamed finds a factory sound of the kind by name.
func FactorySoundNamed(kind Kind, name string) (SoundPreset, bool) {
	for _, p := range FactorySounds(kind) {
		if p.Name == name {
			return p, true
		}
	}
	return SoundPreset{}, false
}

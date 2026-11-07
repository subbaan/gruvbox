package groove

// acidBank adds the 303 sound bank (alpha.37): original patches covering a
// wide range, each a voice setting plus its own pedal chain and sometimes an
// LFO. They follow the four original factory sounds so preset indices stay
// stable. lfo sets the first LFO of the patch just added.
//
// Acid LFO targets: 1 cutoff, 2 resonance, 3 pitch, 4 envelope, 5 pulse
// width, 6 volume. Synced LFO divisions are in bars.
func acidBank(add func(string, string, map[string]float64, ...Effect), effect func(EffectKind, map[string]float64) Effect, lfo func(LFO)) {
	synced := func(target int, division, depth float64, waveform int) {
		lfo(LFO{Enabled: true, Target: target, Waveform: waveform, Rate: 1, Depth: depth, Sync: 1, Division: division, Steps: 16})
	}

	// Squelch and classic acid.
	add("Night squelch", "Low cutoff, screaming resonance and a full accent sweep: runs of accents climb higher and higher.",
		map[string]float64{"cutoff": .22, "resonance": .88, "env": .7, "decay": .35, "accent": .8, "sweep": .9, "sub": .2})
	add("Chirp", "Tiny, fast envelope snaps on a closed filter. Short gate; best on a busy line with accents.",
		map[string]float64{"cutoff": .15, "resonance": .9, "env": 1, "decay": .08, "accent": .9, "gate": .4, "sweep": .6, "sub": .15},
		effect(Delay, map[string]float64{"division": .1875, "feedback": .3, "mix": .15}))
	add("Laser tag", "Bright zaps: full envelope, maximum sweep and a fast flanger shimmering on top.",
		map[string]float64{"cutoff": .55, "resonance": .85, "env": .95, "decay": .12, "accent": 1, "gate": .5, "sweep": 1, "sub": .1},
		effect(Flanger, map[string]float64{"rate": .8, "depth": .6, "manual": 1.2, "feedback": .6, "mix": .4}))
	add("Glass acid", "Clean, glassy resonance with chorus and a wide room.",
		map[string]float64{"cutoff": .5, "resonance": .72, "env": .5, "decay": .3, "accent": .6},
		effect(Chorus, map[string]float64{"rate": .35, "depth": .5, "mix": .25}), effect(Reverb, map[string]float64{"room": .7, "damping": .5, "mix": .18}))

	// Driven.
	add("Fizz acid", "Fuzz-box acid: thick, buzzy and squashed, with a little air around it.",
		map[string]float64{"cutoff": .4, "resonance": .75, "env": .8, "decay": .3, "accent": .7, "drive": .3, "sub": .15},
		effect(Fuzz, map[string]float64{"fuzz": .7, "tone": .45}), effect(Reverb, map[string]float64{"room": .3, "damping": .7, "mix": .08}))
	add("Screamer", "Hot voice drive into a distortion pedal; resonance peaks tear. Dotted echoes behind.",
		map[string]float64{"cutoff": .45, "resonance": .9, "env": .9, "decay": .45, "accent": .9, "drive": .75, "sweep": .7},
		effect(Distortion, map[string]float64{"drive": 3, "tone": .6, "mix": .3}), effect(Delay, map[string]float64{"division": .1875, "feedback": .35, "mix": .15}))
	add("Molten", "Saturated all the way through: voice drive, a dark fuzz and warm tape repeats.",
		map[string]float64{"cutoff": .3, "resonance": .8, "env": .6, "decay": .5, "drive": .9, "sweep": .4, "sub": .25},
		effect(Fuzz, map[string]float64{"fuzz": .5, "tone": .3}), effect(TapeEcho, map[string]float64{"division": .1875, "feedback": .4, "wow": .3, "tone": .35, "mix": .2}))
	add("Grit pump", "Squeezed hard by a compressor, then distorted: every note punches forward.",
		map[string]float64{"cutoff": .38, "resonance": .6, "env": .7, "decay": .25, "drive": .5, "accent": .7},
		effect(Compressor, map[string]float64{"threshold": -28, "ratio": 10, "attack": 4, "release": 90, "makeup": 5}), effect(Distortion, map[string]float64{"drive": 3, "tone": .5, "mix": .45}))

	// Modulated.
	add("Phase squeal", "Light overdrive into a slow, deep phaser; the resonance swims through the notches.",
		map[string]float64{"cutoff": .35, "resonance": .85, "env": .65, "decay": .4, "sweep": .5},
		effect(Overdrive, map[string]float64{"drive": .35, "tone": .6}), effect(Phaser, map[string]float64{"rate": .15, "depth": .8, "feedback": .6, "stages": 1, "mix": .5}))
	add("Jet acid", "A slow, high-feedback flanger turns the line into a jet sweep.",
		map[string]float64{"cutoff": .45, "resonance": .8, "env": .7, "decay": .3, "accent": .7},
		effect(Flanger, map[string]float64{"rate": .1, "depth": .8, "manual": 2, "feedback": .75, "mix": .5}))
	add("Wobble", "An eighth-note cutoff LFO with long notes: a rolling, wobbling bass.",
		map[string]float64{"cutoff": .3, "resonance": .7, "env": .3, "decay": .7, "sub": .4},
		effect(Overdrive, map[string]float64{"drive": .25, "tone": .5}))
	synced(1, .125, .35, 0)
	add("Tremble", "A sixteenth-note volume tremolo chops each note into a stutter.",
		map[string]float64{"cutoff": .4, "resonance": .6, "env": .5, "decay": .45, "accent": .6})
	synced(6, 1./16, .8, 1)
	add("PWM drift", "Square wave whose pulse width drifts over two bars, with chorus: hollow, then nasal, then hollow.",
		map[string]float64{"square": 1, "pulse": .5, "cutoff": .45, "resonance": .5, "env": .45, "decay": .4, "sub": .3},
		effect(Chorus, map[string]float64{"rate": .25, "depth": .5, "mix": .25}))
	synced(5, 2, .4, 0)

	// Wahs.
	add("Wah walker", "Auto-wah on a soft voice: harder notes and accents open the band-pass further.",
		map[string]float64{"cutoff": .42, "resonance": .5, "env": .45, "decay": .5, "accent": .8, "gate": .85},
		effect(EnvelopeFilter, map[string]float64{"shape": 1, "sensitivity": .7, "cutoff": .2, "range": .85, "resonance": .75, "decay": 100}), effect(Overdrive, map[string]float64{"drive": .2, "tone": .6, "level": .8}))
	add("Falling sigh", "An envelope filter that closes as you play harder, plus chorus: sighing, sinking notes.",
		map[string]float64{"cutoff": .4, "resonance": .6, "env": .4, "decay": .6, "slide": .15},
		effect(EnvelopeFilter, map[string]float64{"direction": 1, "sensitivity": .65, "cutoff": .3, "range": .75, "resonance": .6, "decay": 300}), effect(Chorus, map[string]float64{"rate": .3, "depth": .4, "mix": .2}))
	add("Swamp", "Murky and slow: long slides, a low auto-wah and dark tape.",
		map[string]float64{"cutoff": .2, "resonance": .75, "env": .45, "decay": .6, "slide": .2, "drive": .3, "sub": .4},
		effect(EnvelopeFilter, map[string]float64{"sensitivity": .6, "cutoff": .1, "range": .6, "resonance": .7, "decay": 250}), effect(TapeEcho, map[string]float64{"division": .25, "feedback": .45, "wow": .5, "tone": .25, "mix": .2}))

	// Bass and dub.
	add("Rubber worm", "Bouncy and round: long slides, a medium envelope and a touch of chorus. Add slides between notes.",
		map[string]float64{"cutoff": .3, "resonance": .7, "env": .5, "decay": .55, "slide": .12, "sub": .35},
		effect(Chorus, map[string]float64{"rate": .4, "depth": .3, "mix": .15}))
	add("Dub pressure", "Deep, soft and heavy, compressed, with long wobbling tape echoes and a room.",
		map[string]float64{"cutoff": .18, "resonance": .35, "env": .3, "decay": .7, "sub": .7},
		effect(Compressor, map[string]float64{"threshold": -20, "ratio": 4, "attack": 10, "release": 180, "makeup": 4}),
		effect(TapeEcho, map[string]float64{"division": .1875, "feedback": .65, "wow": .45, "tone": .35, "drive": .4, "mix": .3}),
		effect(Reverb, map[string]float64{"room": .65, "damping": .5, "mix": .14}))
	add("Soft sub", "Barely any filter movement and a strong sub: a deep, soft bass under everything.",
		map[string]float64{"cutoff": .12, "resonance": .1, "env": .15, "decay": .8, "accent": .3, "sub": .8},
		effect(Compressor, map[string]float64{"threshold": -18, "ratio": 3, "attack": 15, "release": 200, "makeup": 1}))
	add("Cassette bass", "Worn tape: crushed a little, rolled off and warbling.",
		map[string]float64{"cutoff": .35, "resonance": .45, "env": .5, "decay": .4, "sub": .3},
		effect(BitCrusher, map[string]float64{"bits": 10, "rate": 2, "mix": .3}), effect(Filter, map[string]float64{"cutoff": 3000, "resonance": .2}),
		effect(TapeEcho, map[string]float64{"division": .125, "feedback": .3, "wow": .8, "tone": .3, "mix": .15}))

	// Square and leads.
	add("Staccato stab", "Short square stabs with a hollow pulse and eighth-note echoes. Good for electro.",
		map[string]float64{"square": 1, "pulse": .35, "cutoff": .5, "resonance": .55, "env": .7, "decay": .15, "gate": .3, "sub": .2},
		effect(Delay, map[string]float64{"division": .125, "feedback": .3, "mix": .18}))
	add("Hollow pipe", "A square at an even pulse, gentle filter and a small room: woody and round.",
		map[string]float64{"square": 1, "pulse": .5, "cutoff": .35, "resonance": .3, "env": .25, "decay": .5, "sub": .3, "gate": .8},
		effect(Reverb, map[string]float64{"room": .35, "damping": .6, "mix": .12}))
	add("Needle lead", "A thin pulse an octave up with tape echoes: a nasal lead line.",
		map[string]float64{"square": 1, "pulse": .15, "cutoff": .6, "resonance": .65, "env": .5, "decay": .3, "tune": 12, "gate": .6, "sub": 0},
		effect(TapeEcho, map[string]float64{"division": .125, "feedback": .4, "wow": .25, "mix": .25}))
	add("Ring bell", "High resonance on a short square an octave up, phased, with a room: bell-like pings.",
		map[string]float64{"square": 1, "pulse": .25, "cutoff": .65, "resonance": .92, "env": .4, "decay": .25, "tune": 12, "sweep": .3, "sub": 0},
		effect(Phaser, map[string]float64{"rate": .3, "depth": .6, "feedback": .4, "stages": 2, "mix": .45}), effect(Reverb, map[string]float64{"room": .55, "damping": .4, "mix": .2}))

	// Extended modes (alpha.38): saw/square mix, noise, band- and high-pass.
	add("Hollow band", "Band-pass filter on a half saw, half square: nasal and vocal, the envelope moving a narrow band.",
		map[string]float64{"filter_mode": 1, "blend": .5, "cutoff": .35, "resonance": .7, "env": .6, "decay": .35, "accent": .7, "sub": .25},
		effect(Delay, map[string]float64{"division": .1875, "feedback": .3, "mix": .15}))
	add("Paper cut", "High-pass with a breath of noise: thin, clicky and percussive, for lines over a separate bass.",
		map[string]float64{"filter_mode": 2, "noise": .15, "cutoff": .3, "resonance": .6, "env": .5, "decay": .2, "gate": .5, "sub": 0},
		effect(Reverb, map[string]float64{"room": .4, "damping": .5, "mix": .12}))
	add("Breath", "Noise through a low, resonant filter with long decays: a whistling, airy line.",
		map[string]float64{"noise": .6, "cutoff": .25, "resonance": .85, "env": .45, "decay": .55, "sub": .2},
		effect(Chorus, map[string]float64{"rate": .2, "depth": .5, "mix": .25}))
	add("Half and half", "Saw and square mixed with a narrow pulse and light drive: thick and buzzy.",
		map[string]float64{"blend": .8, "pulse": .3, "cutoff": .4, "resonance": .65, "env": .65, "decay": .3, "drive": .3, "sub": .2})
}

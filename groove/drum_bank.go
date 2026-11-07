package groove

// drumBank adds more built-in drum sounds (alpha.38) after each kind's
// original factory sounds, so preset indices stay stable. Randomise sound
// uses them as archetypes. Names and settings are original.
func drumBank(kind Kind, add func(string, string, map[string]float64, ...Effect), effect func(EffectKind, map[string]float64) Effect) {
	switch kind {
	case Kick:
		add("Long boom", "Low, long and round, with a slow pitch fall: for sparse or dub patterns.",
			map[string]float64{"tune": -5, "decay": .85, "drive": .2, "click": .05, "pitch_decay": .045})
		add("Electro zap", "A long laser-like pitch sweep with a hard click.",
			map[string]float64{"tune": 2, "decay": .45, "drive": .5, "click": .3, "pitch_decay": .08})
		add("Soft pillow", "Clean and gentle, no click, lightly compressed: sits under a busy bassline.",
			map[string]float64{"tune": -2, "decay": .3, "drive": 0, "click": 0, "pitch_decay": .015},
			effect(Compressor, map[string]float64{"threshold": -18, "ratio": 3, "attack": 20, "release": 120, "makeup": 2}))
		add("Crunch box", "Overdriven twice, in the voice and through a drive pedal: a dirty, square kick.",
			map[string]float64{"decay": .3, "drive": .9, "click": .35, "pitch_decay": .02},
			effect(Overdrive, map[string]float64{"drive": .5, "tone": .5, "mix": .5}))
		add("Dub boom", "Deep and soft with a faint dotted tape echo trailing behind.",
			map[string]float64{"tune": -7, "decay": .7, "drive": .3, "click": .02, "pitch_decay": .035},
			effect(TapeEcho, map[string]float64{"division": .1875, "feedback": .3, "wow": .3, "tone": .3, "mix": .12}))
	case Snare:
		add("Tight crack", "High, short and almost all snap.",
			map[string]float64{"tune": 4, "snap": .95, "tone": .25, "decay": .07, "tone_decay": .03})
		add("Fat body", "Low tuned, lots of drum tone and a medium tail.",
			map[string]float64{"tune": -5, "snap": .4, "tone": .8, "decay": .18, "tone_decay": .15})
		add("Electro snare", "Bright snap and tone with a small room.",
			map[string]float64{"tune": 2, "snap": .9, "tone": .5, "decay": .14, "tone_decay": .06},
			effect(Reverb, map[string]float64{"room": .35, "damping": .5, "mix": .12}))
		add("Big room", "A long noise tail in a large, bright room.",
			map[string]float64{"snap": .7, "tone": .4, "decay": .3},
			effect(Reverb, map[string]float64{"room": .85, "damping": .3, "mix": .28}))
		add("Lo-fi snare", "Crushed to a gritty sampler sound.",
			map[string]float64{"snap": .6, "tone": .5, "decay": .15},
			effect(BitCrusher, map[string]float64{"bits": 7, "rate": 4, "mix": .4}))
	case ClosedHat:
		add("Glass tick", "Extremely short, bright and metallic.",
			map[string]float64{"decay": .015, "tone": .95, "noise": .1})
		add("Dusty hat", "Noisy and crushed, like an old sample.",
			map[string]float64{"decay": .05, "tone": .5, "noise": .8},
			effect(BitCrusher, map[string]float64{"bits": 9, "rate": 3, "mix": .35}))
		add("Phased hat", "A slow phaser makes sixteenths swirl over the bar.",
			map[string]float64{"decay": .04, "tone": .75, "noise": .4},
			effect(Phaser, map[string]float64{"rate": .2, "depth": .8, "feedback": .5, "stages": 1, "mix": .5}))
		add("Swish", "Longer and airy: almost half open.",
			map[string]float64{"decay": .12, "tone": .6, "noise": .6})
	case OpenHat:
		add("Short open", "A quick open hat that stays out of the way.",
			map[string]float64{"decay": .18, "tone": .7, "noise": .4})
		add("Long sizzle", "Bright and long, ringing into the next beat.",
			map[string]float64{"decay": .8, "tone": .9, "noise": .5})
		add("Jet open", "Flanged: each open hat sweeps like a jet.",
			map[string]float64{"decay": .4, "tone": .7, "noise": .5},
			effect(Flanger, map[string]float64{"rate": .15, "depth": .8, "manual": 1.5, "feedback": .6, "mix": .45}))
		add("Dark wash", "Low, noisy and long, in a soft room.",
			map[string]float64{"decay": .6, "tone": .3, "noise": .8},
			effect(Reverb, map[string]float64{"room": .5, "damping": .7, "mix": .15}))
	case Clap:
		add("Tight clap", "Short, high and dry.",
			map[string]float64{"tune": 3, "decay": .03, "tone": .8, "drive": .2})
		add("Hall clap", "A clap in a big hall.",
			map[string]float64{"decay": .2, "tone": .55, "drive": .2},
			effect(Reverb, map[string]float64{"room": .85, "damping": .4, "mix": .3}))
		add("Dirty clap", "Driven hard and crushed.",
			map[string]float64{"decay": .1, "tone": .6, "drive": .9},
			effect(BitCrusher, map[string]float64{"bits": 8, "rate": 3, "mix": .3}))
		add("Dub clap", "Dotted tape echoes trail off each clap.",
			map[string]float64{"decay": .12, "tone": .5, "drive": .3},
			effect(TapeEcho, map[string]float64{"division": .1875, "feedback": .5, "wow": .4, "tone": .4, "mix": .25}))
	case Rimshot:
		add("Clave", "A high, pure wooden click.",
			map[string]float64{"pitch": 780, "decay": .03, "tone": .95, "snap": .2})
		add("Block", "A hollow, mid-pitched knock.",
			map[string]float64{"pitch": 420, "decay": .07, "tone": .9, "snap": .3})
		add("Echo rim", "Snappy rim with dotted echoes for dub patterns.",
			map[string]float64{"pitch": 600, "decay": .03, "tone": .5, "snap": .8},
			effect(Delay, map[string]float64{"division": .1875, "feedback": .45, "mix": .3}))
	case Tom:
		add("Floor tom", "Low, long and full, with a gentle pitch fall.",
			map[string]float64{"pitch": 60, "pitch_decay": .5, "decay": .9, "tone": .85, "attack": .4})
		add("Electro tom", "A deep pitch sweep with a clean body: the classic drum-machine pew.",
			map[string]float64{"pitch": 110, "pitch_decay": .9, "decay": .45, "tone": 1, "attack": .3})
		add("Bongo", "High, short and slapped.",
			map[string]float64{"pitch": 260, "pitch_decay": .2, "decay": .15, "tone": .9, "attack": .4})
	case Crash:
		add("Ride ping", "Long, bright and very metallic, for ride patterns.",
			map[string]float64{"decay": 1.5, "tone": .9, "metallic": .95})
		add("Trash crash", "Short, rough and crushed.",
			map[string]float64{"decay": 1.2, "tone": .6, "metallic": .4},
			effect(BitCrusher, map[string]float64{"bits": 8, "rate": 2, "mix": .3}))
		add("Washy ride", "A long, dark wash with chorus movement.",
			map[string]float64{"decay": 2.8, "tone": .5, "metallic": .7},
			effect(Chorus, map[string]float64{"rate": .3, "depth": .5, "mix": .3}))
	}
}

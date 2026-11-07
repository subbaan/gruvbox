package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subbaan/gruvbox/audio"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/desktop"
	"github.com/subbaan/gruvbox/effects"
	"github.com/subbaan/gruvbox/sequencer"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/ui"
)

// randomizeInstruments randomizes TB-303 and drum parameters for varied sounds on startup
func randomizeInstruments(engine *audio.Engine) {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Randomize TB-303 #1 (Bass)
	tb303_1 := engine.GetTB303()
	if rand.Float32() < 0.5 {
		tb303_1.SetWaveform(synth.WaveformSaw)
	} else {
		tb303_1.SetWaveform(synth.WaveformSquare)
	}
	tb303_1.SetCutoff(0.4 + rand.Float32()*0.4)      // 0.4-0.8 (mid to bright)
	tb303_1.SetResonance(0.5 + rand.Float32()*0.4)   // 0.5-0.9 (moderate to high)
	tb303_1.SetEnvMod(0.6 + rand.Float32()*0.35)     // 0.6-0.95 (strong modulation)
	tb303_1.SetDecay(0.2 + rand.Float32()*0.4)       // 0.2-0.6 (short to medium)
	tb303_1.SetAccent(0.4 + rand.Float32()*0.4)      // 0.4-0.8
	tb303_1.SetSlideTime(0.05 + rand.Float32()*0.15) // 0.05-0.2 (subtle to moderate)

	// Randomize TB-303 #2 (Lead)
	tb303_2 := engine.GetTB303_2()
	if rand.Float32() < 0.5 {
		tb303_2.SetWaveform(synth.WaveformSaw)
	} else {
		tb303_2.SetWaveform(synth.WaveformSquare)
	}
	tb303_2.SetCutoff(0.5 + rand.Float32()*0.4)      // 0.5-0.9 (brighter than bass)
	tb303_2.SetResonance(0.6 + rand.Float32()*0.35)  // 0.6-0.95 (high resonance)
	tb303_2.SetEnvMod(0.7 + rand.Float32()*0.25)     // 0.7-0.95 (strong modulation)
	tb303_2.SetDecay(0.1 + rand.Float32()*0.3)       // 0.1-0.4 (short for lead)
	tb303_2.SetAccent(0.5 + rand.Float32()*0.4)      // 0.5-0.9
	tb303_2.SetSlideTime(0.08 + rand.Float32()*0.12) // 0.08-0.2

	// Randomize Kick
	kick := engine.GetKick()
	kick.SetTune(-6.0 + rand.Float32()*12.0)         // -6 to +6 semitones
	kick.SetDecay(0.3 + rand.Float32()*0.3)          // 0.3-0.6 (punchy to sustained)
	kick.SetPitchDecay(0.005 + rand.Float32()*0.015) // 0.005-0.02 (quick pitch drop)
	kick.SetDrive(1.0 + rand.Float32()*2.0)          // 1.0-3.0 (subtle to driven)
	kick.SetClickAmount(0.0 + rand.Float32()*0.3)    // 0.0-0.3 (subtle attack)

	// Randomize Snare
	snare := engine.GetSnare()
	snare.SetTune(-4.0 + rand.Float32()*8.0)       // -4 to +4 semitones
	snare.SetSnap(0.3 + rand.Float32()*0.6)        // 0.3-0.9 (moderate to sharp)
	snare.SetToneDecay(0.04 + rand.Float32()*0.08) // 0.04-0.12 (tight to loose)
	snare.SetNoiseDecay(0.1 + rand.Float32()*0.2)  // 0.1-0.3
	snare.SetToneMix(0.2 + rand.Float32()*0.5)     // 0.2-0.7 (more noise, some tone)

	// Randomize Closed Hi-Hat
	hhc := engine.GetHihatClosed()
	hhc.SetDecay(0.03 + rand.Float32()*0.05)  // 0.03-0.08 (tight)
	hhc.SetTone(0.4 + rand.Float32()*0.5)     // 0.4-0.9 (bright)
	hhc.SetNoiseMix(0.6 + rand.Float32()*0.3) // 0.6-0.9 (metallic)

	// Randomize Open Hi-Hat
	hho := engine.GetHihatOpen()
	hho.SetDecay(0.15 + rand.Float32()*0.2)    // 0.15-0.35 (sustained)
	hho.SetTone(0.5 + rand.Float32()*0.4)      // 0.5-0.9 (bright)
	hho.SetNoiseMix(0.7 + rand.Float32()*0.25) // 0.7-0.95 (very metallic)
}

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version information")
	flag.BoolVar(showVersion, "v", false, "Show version information (shorthand)")
	flag.BoolVar(showVersion, "V", false, "Show version information (shorthand)")
	themeOptions := registerThemeFlags()
	tui := flag.Bool("tui", false, "Run the legacy terminal interface")
	noAudio := flag.Bool("no-audio", false, "Open the desktop editor without an audio device")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("Gruvbox v%s - Desktop Groovebox\n", config.Version)
		fmt.Println("Graphical sequencing with TB-303, analog drums and samplers; --tui runs the legacy interface")
		fmt.Println("Support development: " + desktop.KofiURL)
		os.Exit(0)
	}
	if handled, err := configureThemes(themeOptions); err != nil {
		fmt.Fprintf(os.Stderr, "Theme: %v\n", err)
		os.Exit(1)
	} else if handled {
		return
	}
	if !*tui {
		if err := desktop.Run(*themeOptions.dir, *noAudio); err != nil {
			fmt.Fprintf(os.Stderr, "Desktop: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Ensure all required directories exist
	if err := config.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directories: %v\n", err)
		os.Exit(1)
	}
	// Create audio engine
	engine, err := audio.NewEngine()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create audio engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Start audio engine
	if err := engine.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start audio engine: %v\n", err)
		os.Exit(1)
	}

	// Get components
	seq := engine.GetSequencer()
	mixer := engine.GetMixer()

	// Randomize all instrument parameters for varied sounds on each startup
	randomizeInstruments(engine)

	// Configure effects - ALL 8 channels get all 6 effects in same order
	// This ensures consistent UI navigation (Distortion, Delay, Reverb, Pitch, Filter, Crusher)
	// Effects that aren't used are just disabled by default

	// TB-303 #1 (bass) on channel 0 - Distortion + Delay enabled
	bassChannel := mixer.GetChannel(0)
	bassDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	bassDist.SetDrive(3.0)
	bassDist.SetTone(0.6)
	bassDist.SetMix(0.4)
	bassDist.SetEnabled(true)
	bassChannel.Effects.Add(bassDist)

	bassDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	bassDelay.SetDelayTime(0.375)
	bassDelay.SetFeedback(0.35)
	bassDelay.SetMix(0.25)
	bassDelay.SetEnabled(true)
	bassChannel.Effects.Add(bassDelay)

	bassReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	bassReverb.SetEnabled(false) // Disabled by default
	bassChannel.Effects.Add(bassReverb)

	bassPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	bassPitch.SetEnabled(false) // Disabled by default
	bassChannel.Effects.Add(bassPitch)

	bassFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	bassFilter.SetEnabled(false) // Disabled by default
	bassChannel.Effects.Add(bassFilter)

	bassCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	bassCrusher.SetEnabled(false) // Disabled by default
	bassChannel.Effects.Add(bassCrusher)

	// TB-303 #2 (lead) on channel 1 - Delay + Reverb enabled
	leadChannel := mixer.GetChannel(1)
	leadDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	leadDist.SetEnabled(false) // Disabled by default
	leadChannel.Effects.Add(leadDist)

	leadDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	leadDelay.SetDelayTime(0.1875)
	leadDelay.SetFeedback(0.45)
	leadDelay.SetMix(0.35)
	leadDelay.SetEnabled(true)
	leadChannel.Effects.Add(leadDelay)

	leadReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	leadReverb.SetRoomSize(0.6)
	leadReverb.SetDamping(0.4)
	leadReverb.SetMix(0.2)
	leadReverb.SetEnabled(true)
	leadChannel.Effects.Add(leadReverb)

	leadPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	leadPitch.SetEnabled(false) // Disabled by default
	leadChannel.Effects.Add(leadPitch)

	leadFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	leadFilter.SetEnabled(false) // Disabled by default
	leadChannel.Effects.Add(leadFilter)

	leadCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	leadCrusher.SetEnabled(false) // Disabled by default
	leadChannel.Effects.Add(leadCrusher)

	// Kick on channel 2 - Reverb enabled
	kickChannel := mixer.GetChannel(2)
	kickDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	kickDist.SetEnabled(false)
	kickChannel.Effects.Add(kickDist)

	kickDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	kickDelay.SetEnabled(false)
	kickChannel.Effects.Add(kickDelay)

	kickReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	kickReverb.SetRoomSize(0.3)
	kickReverb.SetDamping(0.7)
	kickReverb.SetMix(0.15)
	kickReverb.SetEnabled(true)
	kickChannel.Effects.Add(kickReverb)

	kickPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	kickPitch.SetEnabled(false) // Disabled by default
	kickChannel.Effects.Add(kickPitch)

	kickFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	kickFilter.SetEnabled(false) // Disabled by default
	kickChannel.Effects.Add(kickFilter)

	kickCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	kickCrusher.SetEnabled(false) // Disabled by default
	kickChannel.Effects.Add(kickCrusher)

	// Snare on channel 3 - Reverb enabled
	snareChannel := mixer.GetChannel(3)
	snareDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	snareDist.SetEnabled(false)
	snareChannel.Effects.Add(snareDist)

	snareDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	snareDelay.SetEnabled(false)
	snareChannel.Effects.Add(snareDelay)

	snareReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	snareReverb.SetRoomSize(0.5)
	snareReverb.SetDamping(0.5)
	snareReverb.SetMix(0.3)
	snareReverb.SetEnabled(true)
	snareChannel.Effects.Add(snareReverb)

	snarePitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	snarePitch.SetEnabled(false) // Disabled by default
	snareChannel.Effects.Add(snarePitch)

	snareFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	snareFilter.SetEnabled(false) // Disabled by default
	snareChannel.Effects.Add(snareFilter)

	snareCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	snareCrusher.SetEnabled(false) // Disabled by default
	snareChannel.Effects.Add(snareCrusher)

	// HH Closed on channel 4 - All disabled by default
	hhcChannel := mixer.GetChannel(4)
	hhcDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	hhcDist.SetEnabled(false)
	hhcChannel.Effects.Add(hhcDist)

	hhcDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	hhcDelay.SetEnabled(false)
	hhcChannel.Effects.Add(hhcDelay)

	hhcReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	hhcReverb.SetEnabled(false)
	hhcChannel.Effects.Add(hhcReverb)

	hhcPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	hhcPitch.SetEnabled(false) // Disabled by default
	hhcChannel.Effects.Add(hhcPitch)

	hhcFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	hhcFilter.SetEnabled(false) // Disabled by default
	hhcChannel.Effects.Add(hhcFilter)

	hhcCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	hhcCrusher.SetEnabled(false) // Disabled by default
	hhcChannel.Effects.Add(hhcCrusher)

	// HH Open on channel 5 - All disabled by default
	hhoChannel := mixer.GetChannel(5)
	hhoDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	hhoDist.SetEnabled(false)
	hhoChannel.Effects.Add(hhoDist)

	hhoDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	hhoDelay.SetEnabled(false)
	hhoChannel.Effects.Add(hhoDelay)

	hhoReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	hhoReverb.SetEnabled(false)
	hhoChannel.Effects.Add(hhoReverb)

	hhoPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	hhoPitch.SetEnabled(false) // Disabled by default
	hhoChannel.Effects.Add(hhoPitch)

	hhoFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	hhoFilter.SetEnabled(false) // Disabled by default
	hhoChannel.Effects.Add(hhoFilter)

	hhoCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	hhoCrusher.SetEnabled(false) // Disabled by default
	hhoChannel.Effects.Add(hhoCrusher)

	// Clap on channel 6 - All disabled by default
	clapChannel := mixer.GetChannel(6)
	clapDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	clapDist.SetEnabled(false)
	clapChannel.Effects.Add(clapDist)

	clapDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	clapDelay.SetEnabled(false)
	clapChannel.Effects.Add(clapDelay)

	clapReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	clapReverb.SetEnabled(false)
	clapChannel.Effects.Add(clapReverb)

	clapPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	clapPitch.SetEnabled(false)
	clapChannel.Effects.Add(clapPitch)

	clapFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	clapFilter.SetEnabled(false)
	clapChannel.Effects.Add(clapFilter)

	clapCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	clapCrusher.SetEnabled(false)
	clapChannel.Effects.Add(clapCrusher)

	// Rimshot on channel 7 - All disabled by default
	rimshotChannel := mixer.GetChannel(7)
	rimshotDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	rimshotDist.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotDist)

	rimshotDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	rimshotDelay.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotDelay)

	rimshotReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	rimshotReverb.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotReverb)

	rimshotPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	rimshotPitch.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotPitch)

	rimshotFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	rimshotFilter.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotFilter)

	rimshotCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	rimshotCrusher.SetEnabled(false)
	rimshotChannel.Effects.Add(rimshotCrusher)

	// Crash on channel 8 - All disabled by default
	crashChannel := mixer.GetChannel(8)
	crashDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	crashDist.SetEnabled(false)
	crashChannel.Effects.Add(crashDist)

	crashDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	crashDelay.SetEnabled(false)
	crashChannel.Effects.Add(crashDelay)

	crashReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	crashReverb.SetEnabled(false)
	crashChannel.Effects.Add(crashReverb)

	crashPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	crashPitch.SetEnabled(false)
	crashChannel.Effects.Add(crashPitch)

	crashFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	crashFilter.SetEnabled(false)
	crashChannel.Effects.Add(crashFilter)

	crashCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	crashCrusher.SetEnabled(false)
	crashChannel.Effects.Add(crashCrusher)

	// Tom on channel 9 - All disabled by default
	tomChannel := mixer.GetChannel(9)
	tomDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	tomDist.SetEnabled(false)
	tomChannel.Effects.Add(tomDist)

	tomDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	tomDelay.SetEnabled(false)
	tomChannel.Effects.Add(tomDelay)

	tomReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	tomReverb.SetEnabled(false)
	tomChannel.Effects.Add(tomReverb)

	tomPitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	tomPitch.SetEnabled(false)
	tomChannel.Effects.Add(tomPitch)

	tomFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	tomFilter.SetEnabled(false)
	tomChannel.Effects.Add(tomFilter)

	tomCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	tomCrusher.SetEnabled(false)
	tomChannel.Effects.Add(tomCrusher)

	// Sampler #1 on channel 10 - All disabled by default
	samp1Channel := mixer.GetChannel(10)
	samp1Dist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	samp1Dist.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Dist)

	samp1Delay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	samp1Delay.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Delay)

	samp1Reverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	samp1Reverb.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Reverb)

	samp1Pitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	samp1Pitch.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Pitch)

	samp1Filter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	samp1Filter.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Filter)

	samp1Crusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	samp1Crusher.SetEnabled(false)
	samp1Channel.Effects.Add(samp1Crusher)

	// Sampler #2 on channel 11 - All disabled by default
	samp2Channel := mixer.GetChannel(11)
	samp2Dist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	samp2Dist.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Dist)

	samp2Delay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	samp2Delay.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Delay)

	samp2Reverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	samp2Reverb.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Reverb)

	samp2Pitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	samp2Pitch.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Pitch)

	samp2Filter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	samp2Filter.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Filter)

	samp2Crusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	samp2Crusher.SetEnabled(false)
	samp2Channel.Effects.Add(samp2Crusher)

	// Drone on channel 12 - All disabled by default
	droneChannel := mixer.GetChannel(12)
	droneDist := effects.NewDistortion(float32(audio.SampleRate), audio.BufferSize)
	droneDist.SetEnabled(false)
	droneChannel.Effects.Add(droneDist)

	droneDelay := effects.NewDelay(float32(audio.SampleRate), audio.BufferSize, 1.0)
	droneDelay.SetEnabled(false)
	droneChannel.Effects.Add(droneDelay)

	droneReverb := effects.NewReverb(float32(audio.SampleRate), audio.BufferSize)
	droneReverb.SetEnabled(false)
	droneChannel.Effects.Add(droneReverb)

	dronePitch := effects.NewPitchShifter(float32(audio.SampleRate), audio.BufferSize)
	dronePitch.SetEnabled(false)
	droneChannel.Effects.Add(dronePitch)

	droneFilter := effects.NewMultimodeFilter(float32(audio.SampleRate), audio.BufferSize)
	droneFilter.SetEnabled(false)
	droneChannel.Effects.Add(droneFilter)

	droneCrusher := effects.NewBitCrusher(float32(audio.SampleRate), audio.BufferSize)
	droneCrusher.SetEnabled(false)
	droneChannel.Effects.Add(droneCrusher)

	// Load a simple demo pattern (user can edit it in the TUI)
	tb303Track := seq.GetTrack(sequencer.InstrumentTB303_1)
	tb303Track.Pattern.SetStep(0, sequencer.Step{
		Active: true, Note: 36, Velocity: 0.9, Accent: true, Probability: 1.0,
	})
	tb303Track.Pattern.SetStep(4, sequencer.Step{
		Active: true, Note: 48, Velocity: 0.7, Slide: true, Probability: 1.0,
	})

	// Kick on the 1s
	kickTrack := seq.GetTrack(sequencer.InstrumentKick)
	for i := 0; i < 16; i += 4 {
		kickTrack.Pattern.SetStep(i, sequencer.Step{
			Active: true, Velocity: 0.9, Probability: 1.0,
		})
	}

	// Set initial tempo
	seq.SetBPM(128.0)

	// Initialize LFO tempo sync on all instruments
	engine.UpdateAllLFOTempo()

	// Create and launch TUI
	model := ui.NewModel(engine, config.Version)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

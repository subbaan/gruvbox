# Gruvbox MegaSynth - Feature Roadmap

## Gruvbox 3 desktop milestones

- [x] Base16 loader, shared semantic colours and persistent theme selection
- [x] Dynamic built-in tracks, capabilities and parameter definitions
- [x] Versioned project format and undo/redo history
- [x] Immutable GUI-to-audio publication and allocation-free render test
- [x] Native desktop transport, graphical step grid and contextual action deck
- [x] Basic piano roll, step details, p-locks, probability and loop conditions
- [x] Theme Conf panel, imports, live preview and cached GitHub catalogue
- [x] Movable native utility windows, with cancellation on close
- [x] Live sliders for sound, FX, LFOs, mixer and steps, with one undo per gesture
- [x] Legacy sound randomisation, expanded sound controls and dual LFOs with free/tempo/step sync
- [x] Euclidean rhythm previews with 1–64 steps, pulses, rotation, layering and undo
- [x] Motif-based 1/2/4-bar phrase generation: Hypnotic/Acid/Rolling recipes, key/scale/register, density/movement/variation/feel, independent rerolls, pitch-only/feel-only modes, contextual slides/gates and exact preview Apply with undo
- [x] Parameter-lock generators for instrument and FX controls, native ranges, step scopes, stable random previews/reroll, humanisation and targeted clearing with undo
- [x] Clap, Rimshot, Tom and Crash GUI voices, sound discovery, locks, dual LFOs and role-specific generators
- [x] New Dual synth mono foundation: two oscillators/sub/noise, driven 12/24 dB filter, independent ADSRs, glide/legato, dual LFOs, locks, randomisation and six presets (do not port MegaSynth)
- [ ] Chord-aware piano roll and Dual synth polyphony
- [x] Dynamic sampler tracks, embedded WAVs, waveform trim, pitched/stereo playback, reverse and one-shot/gate/loop modes
- [x] Non-destructive sample gain, stereo normalization and trim-aware fades, saved with undo
- [x] Independent sample audition and waveform zoom/pan with accurate trim markers
- [x] Linux sample recording from system-output monitors or inputs, mono/stereo, level/clipping meters, bounded takes, trim preview, audition, undoable load and atomic original WAV save
- [x] Ordered stereo effect chains, bypass, saved settings, randomisation and step locks
- [x] Chorus and bit crusher, effect randomisation and two LFOs per effect with free/tempo/step sync
- [x] Searchable sound preset library, 30 factory sounds, sound/FX recall, embedded sampler WAVs and portable import/export
- [ ] Additional FX types and wider modulation routing
- [x] Saved song sections with bar repeats, duplication/reordering and quantised pattern switching
- [x] Section mute overrides, mixer snapshots, whole-groove copies and empty-slot variations
- [x] Explicit whole-pattern control storage/recall with Store state and Ctrl+Shift+P: mute/solo/mix, instrument/FX LFOs and sounds, sampler assets/processing, independent copies, undo, quantised audio recall, song/export integration and clean editor following
- [x] Background loop/song WAV export with effect tails, progress, cancellation and atomic replacement
- [x] One-bar queued drum fills and track/groove variations, cancellation and keeping takes with undo
- [ ] Broader performance macros and mute gestures
- [x] Pitched Tom piano-roll editing, bass tone preset and bassline generation
- [x] Piano-roll note dragging and octave navigation, preserving note details with undo
- [x] Piano-roll note lengths, Shift-click multi-selection, phrase move/duplicate/transpose/delete and undo
- [ ] VST3/CLAP hosting
- [x] New-project dropdown with classic acid, full drum machine, Tom bass, sampler kit and empty templates
- [ ] Broader classic instrument templates
- Gruvbox 2 importer is out of scope: the sole user does not need it.

The following roadmap describes the retained legacy TUI work.

## ✅ Completed (v2.15.0)

### Core Synth Enhancements
- [x] Renamed Drone Synth → MegaSynth
- [x] 3 independent oscillators with waveform selection (Saw/Square/Triangle/Sine)
- [x] Pulse width modulation per oscillator (0-100%)
- [x] Unison mode parameters (voices 1-7, detune, stereo width)
- [x] Individual waveform selection per oscillator
- [x] All parameters properly mapped and adjustable in UI
- [x] Updated UI with emoji header 🎛️

### Arpeggiator Backend
- [x] Arpeggiator engine created
- [x] Pattern modes: Up, Down, UpDown, DownUp, Random, AsPlayed
- [x] Rate control (1/32, 1/16T, 1/16, 1/8T, 1/8, 1/4)
- [x] Gate length, octave range (1-4), swing parameters
- [x] Latch mode support
- [x] Integrated into MegaSynth (accessible via GetArpeggiator())

### Version Management
- [x] Updated VERSION to 2.15.0
- [x] All parameter indices aligned and tested

---

## 🚧 In Progress

### Arpeggiator View
- [ ] Add ViewArp to ViewMode enum
- [ ] Create dedicated Arpeggiator view UI
- [ ] Show arp pattern visually
- [ ] Display input chord notes
- [ ] Live sequence preview
- [ ] Keyboard controls for all arp parameters
- [ ] Wire arpeggiator to sequencer note events

---

## 📋 High Priority - Next Up

### Unison Audio Implementation
- [ ] Implement actual voice spreading in Process()
- [ ] Detune multiple voices per note
- [ ] Stereo width panning
- [ ] Pre-allocate voice buffers (real-time safe)
- [ ] Test with different voice counts (2-7)

### Beautiful Sectioned UI
- [ ] Redesign MegaSynth view with sections
- [ ] Section 1: Oscillators (with visual progress bars)
- [ ] Section 2: Filter & Envelope (with ADSR preview)
- [ ] Section 3: Modulation (LFO1 & LFO2)
- [ ] Navigate sections with ←/→ keys
- [ ] Add visual flair (progress bars, unicode art)

### Preset System
- [ ] Create preset save/load infrastructure
- [ ] JSON-based preset format (.megasynth files)
- [ ] Factory presets (Acid Bass, Techno Lead, etc.)
- [ ] User preset directory (presets/megasynth/)
- [ ] Preset browser view
- [ ] Quick save/load shortcuts

---

## 🎯 Medium Priority - Future Enhancements

### Filter Enhancements
- [ ] Add filter type selection (LP/HP/BP/Notch)
- [ ] Use MultimodeFilter from effects/
- [ ] Switchable between TB-303 ladder and SVF filters
- [ ] Filter type parameter in UI

### Modulation Matrix
- [ ] Route any mod source to any destination
- [ ] LFO → Filter, Pitch, Pan, Volume, etc.
- [ ] Envelope → multiple targets
- [ ] Velocity sensitivity routing
- [ ] UI for matrix (grid or list view)

### Additional Oscillator Features
- [ ] Oscillator sync (hard/soft)
- [ ] Waveshaper variants (different saturation curves)
- [ ] More sub-oscillator waveforms
- [ ] Noise color selection (white/pink/brown)

### Performance Features
- [ ] Macro controls (map multiple params to one knob)
- [ ] 4 user-definable macros
- [ ] Randomization with constraints
- [ ] Morph between two presets

---

## 💡 Low Priority - Nice to Have

### Advanced Arpeggiator
- [ ] Custom arp patterns (user-defined step sequences)
- [ ] Arp pattern editor
- [ ] Velocity patterns
- [ ] Probability per arp step

### Voice Management
- [ ] Polyphony mode (multiple simultaneous notes)
- [ ] Voice allocation strategy (oldest/newest/quietest)
- [ ] Legato/portamento modes
- [ ] MPE support (if hardware available)

### Effects Integration
- [ ] Built-in chorus for MegaSynth channel
- [ ] Phaser effect option
- [ ] Compressor/limiter per voice
- [ ] Sidechain from kick to MegaSynth

### UI Polish
- [ ] Waveform visualizer (show osc output)
- [ ] Real-time spectrum analyzer
- [ ] Envelope shape preview (ADSR graph)
- [ ] LFO phase indicator
- [ ] VU meters per oscillator

### Project Integration
- [ ] Save MegaSynth settings in .gruvbox projects
- [ ] Pattern-based automation (already supported via p-locks)
- [ ] MIDI learn for external controller support

---

## 🐛 Known Issues / Tech Debt

- [ ] Unison parameters exist but don't affect audio yet
- [ ] Arpeggiator backend exists but no UI to trigger it
- [ ] Parameter count is getting high (51 params) - sectioned UI will help

---

## 🎼 Sound Design Presets (To Create)

Once preset system is built, create these factory presets:

### Bass
- [ ] Acid Bass (classic TB-303 style)
- [ ] Sub Bass (pure sine sub)
- [ ] Reese Bass (detuned saws with unison)
- [ ] Wobble Bass (fast LFO on filter)

### Leads
- [ ] Techno Lead (aggressive saw lead)
- [ ] Pluck (short envelope, bright)
- [ ] Supersaw (unison 7 voices, detuned)

### Pads
- [ ] Dark Pad (slow attack, rich harmonics)
- [ ] Lush Pad (triangle waves, chorus-like detune)
- [ ] String Pad (multiple saws, slow movement)

### FX
- [ ] Industrial Drone (original use case!)
- [ ] Rumble (low freq, ring mod, noise)
- [ ] Siren (pitch LFO, fast rate)

---

## 📝 Notes

- Keep real-time audio safety in mind for all audio processing
- Pre-allocate all buffers
- No locks or allocations in Process()
- Test with high CPU load to verify no dropouts
- Follow existing Gruvbox color scheme and TUI patterns

---

**Current Version:** 2.15.0
**Last Updated:** 2026-01-04
**Next Milestone:** Arpeggiator View (v2.16.0)

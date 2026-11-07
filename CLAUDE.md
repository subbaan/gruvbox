# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gruvbox is a desktop and terminal groovebox featuring TB-303 bass synthesizer emulation, analog-modeled drum synthesis, step sequencer, and per-channel effects. Current version: 3.0.0-alpha.51

## Desktop UI redesign (in progress)

The desktop GUI (`desktop/`) is being reworked in phases across several sessions.
**Read `UI_REDESIGN_PLAN.md` before changing anything in `desktop/`.** It holds the
audit, the phase checklist, the next task, conventions (including headless
screenshots via `GRUVBOX_SHOTS=<dir> go test ./desktop -run TestScreenshots`) and a
session log. Update it at the end of every session.

## Sound variety work (complete, alpha.41)

Generation, sounds, the 303 voice and effects were extended in four phases.
**Read `SOUND_VARIETY_PLAN.md` before changing `groove/` generation,
`synth/` or `effects/`**: it records where each piece lives, the level and
default-compatibility rules, and possible follow-ups.

## Track builder work (complete, alpha.47)

Hold parts, Grow into a track, Jam mode and Groove history. **Read
`TRACK_BUILDER_PLAN.md` before changing randomise, pattern slots or song
generation**: it records the design, the playback facts it relies on and a
session log.

## Repository

https://github.com/subbaan/gruvbox (private), branch `main`; Go module
`github.com/subbaan/gruvbox`. Releases are tagged `v<VERSION>` as
pre-releases with `gruvbox-linux-x86_64` attached. Only what is needed to
build is checked in; see `.gitignore`.

## Pattern banks (alpha.51) and Jam → Song (next)

64 pattern slots as 8 banks (A–H) of 8 pads, format version 2. **Read
`PATTERN_BANKS_PLAN.md` before changing pattern slots, the project file
format, `Publish`'s banks or Jam recording.**

## Version Management

CRITICAL: Always update the VERSION file when making code changes. The version follows semantic versioning (MAJOR.MINOR.PATCH).

## Build Commands

```bash
# Install dependencies
go mod tidy

# Build the application
go build -o gruvbox

# Run the application
./gruvbox

# Show version
./gruvbox --version
```

## Architecture Overview

### Real-Time Audio Thread Safety

The audio engine follows strict real-time audio programming principles. The `Engine.Read()` method is the core audio callback and must NEVER:
- Perform dynamic memory allocation (malloc/new)
- Use mutex locks
- Do I/O operations (printf, file I/O)
- Use blocking channel operations

All buffers are pre-allocated. Communication between UI thread and audio thread uses lock-free ring buffers (`util/ring_buffer.go`).

### Core Components

**Audio Engine** (`audio/engine.go`):
- Real-time audio callback via oto v3 integration
- Manages all synthesizers, drums, samplers, and the sequencer
- Pre-allocated buffers to avoid GC in audio thread
- Sample rate: 44100 Hz, Buffer size: 256 samples (~5.8ms latency)

**Mixer** (`audio/mixer.go`):
- 8 independent channels with pre-allocated buffers
- Per-channel effects chains (distortion, delay, reverb)
- Level, pan, mute, solo controls
- Constant-power panning and brick-wall limiter

**TB-303 Synthesizer** (`synth/tb303.go`):
- Anti-aliased oscillators using polyBLEP (sawtooth & square)
- Diode ladder filter (24dB/oct) with topology-preserving transform
- Decay envelope generators with exponential curves
- Accent, slide/portamento, sub-oscillator, and dual LFOs
- Two independent instances (bass + lead)

**Drum Synthesis** (`synth/drums/`):
- Analog-modeled kick, snare, and hi-hats
- Each drum has dedicated voice implementation
- Integrated with sequencer via `drums.DrumVoice` interface

**Sequencer** (`sequencer/`):
- Sample-accurate clock with BPM control (40-300 BPM)
- Step sequencer with 16/32/64 step patterns
- Pattern bank system: 16 patterns per instrument (96 total patterns)
- Song mode for arranging patterns into complete tracks
- Each step stores: note, velocity, accent, slide, probability

**Effects** (`effects/`):
- Effect interface and chain manager for serial processing
- Distortion, delay (with tone filter), reverb (Freeverb algorithm)
- Per-channel effects chains integrated with mixer
- All effects have dry/wet mix control

**Project Management** (`project/`):
- Complete state serialization to JSON (.gruvbox files)
- Stores all patterns, synth/drum settings, effects, mixer state
- Projects saved to `projects/` directory
- Human-readable format for version control

**Terminal UI** (`ui/`):
- Bubble Tea framework for TUI
- Multiple views: Sequencer, Synth, Drums, Effects, Mixer, Song, Sampler
- Real-time parameter editing without audio glitches
- Gruvbox color scheme styling

### Key Architectural Patterns

1. **Lock-Free Communication**: UI → Audio thread uses ring buffers, never locks
2. **Pre-Allocation**: All audio buffers allocated at startup to avoid GC pressure
3. **Pattern Bank System**: Each instrument has 16 pattern slots that store both step data AND sound settings
4. **Effect Chains**: Effects are organized in serial chains per mixer channel
5. **Sample-Accurate Timing**: Sequencer clock generates precise timing events

## Development Workflow

When adding features:
1. Update VERSION file first
2. For audio path changes, ensure no allocations in `Engine.Read()`
3. For UI changes, use Bubble Tea's message passing (never call audio engine directly from keyboard handlers)
4. For new parameters, add to ring buffer communication if UI needs to control them
5. Test with audio running to ensure no dropouts or glitches

When modifying synthesizers or effects:
- Use pre-allocated buffers passed to `Process()` methods
- Avoid `math` package in audio thread (use `util/math.go` fast approximations)
- Test with high CPU load to verify real-time performance

## Audio Configuration

- Sample rate: 44100 Hz (constant defined in `audio/constants.go`)
- Buffer size: 256 samples
- Format: Stereo 16-bit signed integer
- Channels: 8 mixer channels (TB303 #1, TB303 #2, Kick, Snare, HH Closed, HH Open, Sampler1, Sampler2)

## Dependencies

- `github.com/ebitengine/oto/v3` - Low-latency cross-platform audio
- `github.com/charmbracelet/bubbletea` - Terminal UI framework
- `github.com/charmbracelet/lipgloss` - TUI styling

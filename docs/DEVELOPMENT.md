# Developing Gruvbox

Gruvbox is a Go program with two front ends: the desktop app (Fyne, the one
users run) and the legacy terminal UI (`--tui`, Bubble Tea). They share the
synth, drum and effect code but have separate models, engines and file
formats.

## Building and running

```bash
go build -trimpath -o gruvbox      # the binary, in the project folder
./gruvbox --no-audio               # desktop app without an audio device
./gruvbox --version
```

You need Go 1.24 or newer, a C compiler, and the OpenGL, X11 and ALSA
development files (see the README). `-trimpath` keeps the build folder out
of the binary.

## Layout

The desktop app:

| Package | What it holds |
|---|---|
| `main.go`, `theme_commands.go` | Flags, theme commands, starting the desktop app or the TUI |
| `desktop/` | The Fyne UI: track grid, piano roll, inspector, drawers, song timeline, file dialog, export and recording windows |
| `groove/` | The project model (`model.go`), undoable sessions, the `.gruvbox3` format (`files.go`), generation (styles, Fit, Grow, Jam, history, phrases, the mono-synth idioms), factory sounds, effects settings, and the audio `Player` (`audio.go`, `master.go`) |
| `synth/` | Voices: TB-303 (`tb303.go`, `tb303_filter.go`), Dual synth (`dual.go`), SH-101 (`sh101.go`) |
| `synth/drums/` | Kick, snare, hats, clap, rimshot, tom, crash |
| `effects/` | Effect DSP: drives, filters, modulation pedals, delay, tape echo, reverb, compressor |
| `dsp/`, `util/` | LFOs, filters, fast math |
| `capture/` | Recording from an input or the system output (`parec`) |
| `theme/` | Base16 themes: bundled, imported, downloaded |
| `config/` | The version string and app constants |

Only the legacy TUI uses `ui/`, `audio/` (its engine and 8-channel mixer),
`sequencer/`, `project/` (`.gruvbox` JSON) and `sampler/`.

## How the desktop app fits together

- **Edits.** Every change goes through a `groove.Session` command
  (`u.edit(label, fn)` in `desktop/`), which keeps undo and redo. Generation
  functions in `groove/` change a `Project` and are called inside such a
  command.
- **Playback.** After an edit the UI calls `Player.Publish(project)`. The
  audio thread picks up the immutable snapshot at the next block boundary;
  voice state belongs to the audio thread. `Player.Render` mixes every
  track's voice and effects, and the master bus adds make-up gain and a
  look-ahead limiter. Playback, track renders and WAV export share this path.
- **Instruments.** A track's `Kind` picks its voice and its controls
  (`groove.Parameters`), stepped switches (`ParameterChoices`) and LFO
  targets (`LFOTargets`). The Dual and the SH-101 share the `monoVoice`
  interface in `groove/audio.go`.
- **Generation.** `groove/style.go` defines the six styles: layout, part
  recipes per role, sounds, effects. `groove/fit.go` writes a part for a
  track's role (`FitPart`), `groove/generate.go` runs the generator
  buttons, `groove/phrase.go` is the phrase engine, and
  `groove/mono_parts.go` writes the Dual's Lead and Stabs and the SH-101's
  Arp and Sequence.

The plan files in the repository root hold the design history and the
reasons behind each area. Read the relevant one before changing that area:

- `UI_REDESIGN_PLAN.md`: the desktop UI, its conventions and headless
  screenshots.
- `SOUND_VARIETY_PLAN.md`: generation, sounds, the voices and effects. Its
  session log covers Fit groove, the SH-101 and the alpha.62 Dual and
  SH-101 work.
- `TRACK_BUILDER_PLAN.md`: Hold, Grow, Jam and groove history.
- `PATTERN_BANKS_PLAN.md`: the 64 pattern slots and the file format.
- `GRUVBOX3_PRODUCT_DESIGN.md`: the overall product design.

## Real-time audio rules

Code that runs on the audio thread (`Player.Render` and everything it calls:
voices, effects, LFOs) must never:

- allocate memory,
- take a lock,
- do I/O (logging, files),
- block on a channel.

Pre-allocate buffers, smooth parameter changes, and test with
`testing.AllocsPerRun` (see `synth/dual_test.go` and `groove/audio_test.go`).
New voice features should leave the default sound unchanged, so old
projects and presets sound as they did.

## Testing

```bash
go vet ./...
go test ./...
go test -race ./capture ./groove ./synth ./effects ./theme
```

Some tests write files to listen to or look at when an environment variable
names a directory:

| Variable | Test | Writes |
|---|---|---|
| `GRUVBOX_SHOTS` | `go test ./desktop -run TestScreenshots` | Headless PNGs of the UI |
| `GRUVBOX_GROOVE_WAVS` | `go test ./groove -run TestWriteStyleSamples` | Grooves in every style |
| `GRUVBOX_MONO_WAVS` | `go test ./groove -run TestWriteMonoPartSamples` | Dual and SH-101 parts in every style, full and solo |
| `GRUVBOX_VOICE_WAVS` | `go test ./groove -run TestWriteVoiceSamples` | 303 character settings |
| `GRUVBOX_BANK_WAVS` | `go test ./groove -run TestWriteBankSamples` | The 303 sound bank |
| `GRUVBOX_PEDAL_WAVS` | `go test ./groove -run TestWritePedalSamples` | A groove through every pedalboard |

`scripts/*-smoke.sh` drive the real app under Xvfb with xdotool and
ImageMagick and write captures to `artifacts/` (not checked in):

```bash
xvfb-run -a -s '-screen 0 1440x1000x24' bash scripts/gui-smoke.sh
```

When driving the real app by hand, send input only to your own process's
window (`xdotool search --pid <pid>`) and stop it with `kill <pid>`: other
windows called "gruvbox" may be someone's terminal or their own copy of the
app.

## Extending

- **A control on an existing instrument:** add it to the voice's parameter
  list (for the Dual, `dualDefinitions` in `synth/dual.go`; append, because
  the index constants follow the list) with a default that keeps the old
  sound. Missing parameters in old files are filled from defaults
  (`fillParameterDefaults`). Add the control to the instrument's panel in
  `desktop/`, and to `RandomiseSound` in `groove/sound.go` if it should be
  explored.
- **An LFO target:** append it to the kind's target list. Saved LFOs store
  the index, so never reorder.
- **A factory sound:** add it in `groove/factory_sounds.go` (or the
  `*_bank.go` files) after the existing ones, and update the count in
  `groove/presets_test.go`. The bank level tests keep new sounds near the
  default's loudness.
- **A generator:** add it to `kindActions` in `groove/generate.go` (and to
  `melodic` if it writes notes in key); the inspector shows the list as
  buttons.
- **A part in grooves:** add a `partRecipe` with a role to the style in
  `groove/style.go`, a label in `roleLabels` (`groove/fit.go`), and say in
  `answers` whether it should play in the bass's gaps.
- **An effect:** see the Phase B notes in `SOUND_VARIETY_PLAN.md`.

## Pitfalls

- Generation should stay seedable: take a `*rand.Rand` and iterate maps in
  sorted order. Adding a random draw shifts seeded tests that depend on later
  draws, so expect to re-pick a seed now and then.
- `Project.Style` and `Track.Role` are `omitempty`: the file format version
  only changes for incompatible changes.
- Envelope times in seconds (`groove.TimeParameter`) vary by ratio, not by a
  share of their range, and show on log sliders.

## Versions and releases

Every change bumps the patch version in `VERSION`, `config/config.go`,
`README.md` and `CLAUDE.md`, with an entry in `CHANGELOG.md`. Releases are
tagged `v<VERSION>`, marked as pre-releases, with the binary attached as
`gruvbox-linux-x86_64` together with `LICENSE`. Before releasing, check that
the binary holds no home paths (`strings -a gruvbox | grep -E '/home/'`
should print nothing).

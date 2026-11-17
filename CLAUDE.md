# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gruvbox is a desktop groovebox (Fyne) with TB-303, Dual synth and SH-101
voices, analog-modelled drums, a sampler, per-track effects, generated
grooves, song mode and WAV export. A legacy terminal UI runs with `--tui`.
Current version: 3.0.0-alpha.62

**Read `docs/DEVELOPMENT.md` first**: package layout, how the desktop app
fits together, the real-time audio rules, tests, how to extend it and how to
release. This file only lists the working rules and where to look.

## Working rules

- Every change bumps the patch version in `VERSION`, `config/config.go`,
  `README.md` and this file, and adds a `CHANGELOG.md` entry.
- Build with `go build -trimpath -o gruvbox` (binary in the project root) and
  run `go vet ./...` and `go test ./...` before handing off.
- Audio-thread code (`Player.Render` and everything it calls) never
  allocates, locks, does I/O or blocks. Test with `testing.AllocsPerRun`.
- New voice controls default to "off", so old projects and presets sound the
  same; append parameters and LFO targets, never reorder them.
- `README.md` is for users. Put developer material in `docs/DEVELOPMENT.md`,
  and keep both current in the same change as the code.
- When driving the real app, send input only to your own process's window
  (`xdotool search --pid`) and stop it with `kill <pid>`, never `pkill`.

## Read before changing

- `desktop/`: **`UI_REDESIGN_PLAN.md`** (conventions, headless screenshots
  via `GRUVBOX_SHOTS=<dir> go test ./desktop -run TestScreenshots`, session
  log; update it at the end of a session).
- `groove/` generation, `synth/`, `effects/`: **`SOUND_VARIETY_PLAN.md`**
  (where each piece lives, level and default-compatibility rules; its
  session log covers Fit groove, the SH-101 and the alpha.62 Dual/SH-101
  split). `Project.Style` and `Track.Role` stay `omitempty`.
- Randomise, pattern slots, song generation: **`TRACK_BUILDER_PLAN.md`**.
- Pattern slots, the file format, `Publish`'s banks, Jam recording:
  **`PATTERN_BANKS_PLAN.md`** (64 slots, A–H × 8, format version 2; Jam →
  Song is the next idea there).

## Repository

https://github.com/subbaan/gruvbox (public, MIT), branch `main`; Go module
`github.com/subbaan/gruvbox`. Releases are tagged `v<VERSION>` as
pre-releases with `gruvbox-linux-x86_64` attached. Only what is needed to
build is checked in; see `.gitignore`.

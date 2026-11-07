# Track builder plan

Started 2026-11-06 at 3.0.0-alpha.41, after the sound variety work
(`SOUND_VARIETY_PLAN.md`). The user asked for all four follow-ups, with
"Grow into a track" as the priority: "it needs to work well".

Order: 1 Hold parts → 2 Grow into a track → 3 Jam mode → 4 Groove history.
Each step bumps the patch version (VERSION, config/config.go, README.md,
CLAUDE.md), builds `go build -o gruvbox`, passes `go test ./...`, and updates
the session log below.

## How playback works (facts the design relies on)

- A song section is `{Name, Pattern, Repeats (bars), Mutes}`. Each section
  start calls `switchPattern`, which resets voices, LFOs and the step counter,
  so every section starts at step 0 of its pattern.
- Patterns are at most 64 steps (4 bars at 1/16). Track lengths can differ.
- Step locks are applied on every step, active or not, and do not latch: the
  next step restores the track value. A lock on every step is a ramp.
- `Pattern.State` (stored sound) is used when that slot plays; a slot with no
  State follows the track's live sound. Selecting a slot with State recalls it.
- 8 pattern slots per project.

## 1. Hold parts (alpha.42) — done

- `Track.Hold bool` (`json:"hold,omitempty"`). An undoable edit, no audio.
- An **H** chip left of M/S in the grid name column, and a track menu item.
- `RandomiseGroove` skips held tracks (pattern and sound). If anything is
  held, BPM and swing are kept; if a held track plays notes (Acid, Dual), the
  key is kept so new lines fit. The status line lists what was held.
- `RandomiseGrooveParts(p, r, part)` rerolls only drums (+ sampler) or only
  the 303s/synths; the other group counts as held.
- UI: a ▾ button beside Randomise all with Randomise all / drums only / 303s
  only; the same three in the ≡ menu.

## 2. Grow into a track (alpha.43) — done

From the current pattern (the "peak"), fill the other 7 slots with related
patterns and lay out a song. One undoable edit; confirm first if any other slot
or the song already has content. Turns song mode on so Play plays the track.

Derived slots (source stays where it is; the others take the remaining slots
in song order):

| Slot | What changes |
|---|---|
| Intro | kick + closed hats only; others empty |
| Build | everything but snare/clap/crash; 303s with a steady low cutoff lock |
| Riser | 4 bars (tiled to 64 steps): cutoff ramp on every step (low → a bit above the peak), a snare roll getting denser (¼ → ⅛ → 1/16 → 1/16 rising), kick drops out on the last beat |
| Peak | the source, untouched |
| Fill | one bar: the source's last bar, with a drum fill on the last beat (toms if present, else snare) |
| Breakdown | no kick/snare/clap/toms/open hats; hats thinned to offbeats; only the lead synth, locked longer decay, lower cutoff, more echo/reverb mix; crash once per 4 bars |
| Peak 2 | source with a mutated synth line, locked extra accent sweep, drive and accent; a few ghost hats |
| Fill 2 | one bar of Peak 2 with the fill |

**No stored sounds (State) in derived slots.** Song mode calls
`FollowPattern`, which recalls a slot's State into the track; slots without
State then play whatever was recalled last. So every sound change is a step
lock on every step; derived slots copy the source's State only if it has one.
`TestFollowingAGrownSongKeepsTheSound` guards this.

Arrangement (Short ≈ 2 min / Full ≈ 4 min at 128 BPM; Full doubles the
lengths of Intro, Build, Peak, Breakdown, Peak 2, Outro):
Intro 8 · Build 8 · Riser 4 · Peak 15 + Fill 1 · Breakdown 8 · Riser 4 ·
Peak 2 15 + Fill 2 1 · Outro (Intro slot) 8.

Notes:
- The fill bar is taken from where the source would be at that bar
  (`(bars-1)*resolution % len`), so it lines up with 1/2/4-bar patterns.
- Riser bars = min(64, 4·resolution)/resolution.
- Locks are absolute values: after a big sound change, Grow again to rebase
  the build/riser/breakdown locks.
- Tests: arrangement validity, slot contents per role, ramps monotonic, fill
  alignment, riser length for odd resolutions, seed repeatability, drums-only
  and 303-only projects, undo. A WAV writer renders the whole track.

## 3. Jam mode (alpha.44) — done

An optional toggle while playing: every 8 bars (4 or 8) a gentle change
(mutate one part, drop/return a part, nudge a cutoff), applied as a normal
edit so undo history keeps every step. Off by default.

## 4. Groove history (alpha.45) — done

The last 20 Randomise results (seed + label + project snapshot of the slot)
in a strip/menu; star to keep favourites (saved in settings). Picking one
restores it as an undoable edit.

## Session log

- 2026-11-06: plan written.
- 2026-11-06, alpha.42: Hold parts done. `Track.Hold`, H chip (holdLeft 124),
  H key, track menu item; `RandomiseGrooveParts` + `GroovePart`
  (`AllParts`/`DrumParts`/`SynthParts`), `RerolledTracks`; ▾ menu beside
  Randomise all and a ≡ → Randomise submenu; Ctrl+Shift+R drums, Ctrl+Alt+R
  303s. Random draws happen in the same order as before, so unheld results
  are unchanged.
- 2026-11-06, alpha.43: Grow into a track done (`groove/grow.go`,
  `desktop/grow.go`). Song tab button, Patterns menu, Ctrl+G; card with
  Short/Full (remembered per run) naming replaced slots. Measured on
  renders (`GRUVBOX_GROW_WAVS`): >1 kHz energy climbs intro → build →
  riser, breakdown 5–12 dB down. Note: RenderWAV's tail (1 s in the test)
  shifts a bar grid computed from file length.
- 2026-11-06, alpha.44: Jam mode done. `Player.BarClock()` (render-side,
  bars since Play; `barsPlayed` counted in `advanceBar`, stored per block).
  `groove.Jam(p, r, *JamState)`: vary (two steps of one part; drum
  downbeats kept; synth notes move to neighbouring scale notes), drop/return
  (never the kick or the last synth; max 2 out; only returns its own drops),
  nudge (303 cutoff/resonance/decay/env within `jamRanges`, leaning to the
  middle). Desktop `desktop/jam.go`: Perform tab toggle + every 4/8/16,
  J key; moves land 1/8 bar early; skipped under a slider drag or open card.
- 2026-11-06, alpha.45: Groove history done. `groove/history.go`:
  `GrooveSnapshot` (tempo, swing, key, per-track pattern + sound; matched
  by kind in order), `CaptureGroove`, `RestoreGroove` (respects Hold like
  randomise), `Load/SaveFavourites` (atomic write).
  `desktop/history.go`: drawer "Groove history" (▾ menu, Ctrl+H) with
  Recent (20, this run, incl. the startup groove) and Favourites at
  `<data root>/favourite-grooves.json`, written only on star/unstar.
- All four features are done.
- 2026-11-07, alpha.46 (user: "the variations are a bit small … it doesn't
  evolve much"; startup should use the newer drums):
  - New grooves have ten tracks: `NewProject`'s six plus `ExtraKinds`
    (Clap, Rimshot, Tom, Crash), colours 6, 7, 2, 5. `NewProject` itself
    stays six. Each style has `extras` (play chance, level, recipe); a
    part that does not play keeps its sound and an empty pattern.
    `shapeDrumPart`: crash keeps only its first hit (plays from the start
    only in loops of 32+ steps); tom hits fall through the key per bar
    (notes 36–48). Randomise all uses the same recipes.
  - Jam: `JamState.Amount` (`JamGentle` = old behaviour, `JamEvolving`
    UI default, `JamWild`), `Moves` and `Rising()` (a 12-change wave: six
    building, six releasing). A change is 1 / 1–2 / 2–3 moves, each on a
    different track (`touched`). New moves: `jamRewrite` (another drum
    family, never the current one via `closestFamily`; kick only when
    Wild), `jamRephrase` (a new phrase in one bar or the end of bar 1,
    octave-matched by `registerShift`, locks kept), `jamBringIn` (silent
    tracks, building only unless Wild), `jamSwapSound` (drum bank sound),
    drum decay/tone and FX mix nudges. Drops mostly while releasing,
    returns while building. Crash is never varied (one hit per loop).
  - `GRUVBOX_JAM_WAVS=<dir> go test ./groove -run TestWriteJams` renders
    96 bars at each amount with a change log.
- 2026-11-07, alpha.47 (user: the extras were "the same ones each time …
  dual synth never gets placed"): layouts are drawn from the pool.
  `GrooveStyle.layout` (`layoutRecipe`: backbeat weights snare/clap/both,
  `dual` chance for the second voice, `optional` weights for Open Hat,
  Rimshot, Tom, Crash) and `GrooveStyle.dual` (recipes `dualCounter`,
  `dualStab`, `dualPad` (hold: notes ring to the next), `dualLayer`
  (action "Layer": `layerLead` doubles the first synth an octave down,
  min note 24)). `chooseLayout`: 303 lead, Kick, Closed Hat; second voice;
  backbeat; 2–4 optional (2 most often; crash only for 32+ steps); 1–2
  silent from the rest (spare synth at half weight). Tracks in
  `trackOrder`; synth colours 0, 1, then the first colour no drum uses.
  `partRecipe.level` sets Dual levels (.28–.36, measured 1–7 dB under the
  lead, like the second 303 at about −4 dB). Randomise all keeps the
  layout and uses `playChance` (clap/snare by backbeat weights when both
  exist, optional drums by weight) and the style's Dual recipes; layers
  are redone after every lead is written. `ExtraKinds` is gone.
  Grow fix: the falling tom fill clamped flat for high toms (pitch ≥ 190);
  the ramp is now clamped first and always falls.
- Possible follow-ups: Grow variants (another arrangement shape, an 8-bar
  breakdown riser), Jam moves that change pattern length or swing,
  naming/renaming favourites, a ride pattern family for the crash track.
- **2026-11-07 (alpha.51):** Pattern banks. Grow now writes the other seven pads of the source's bank (`GrowSlots`), and Variation wraps within the bank (`NextInBank`). See `PATTERN_BANKS_PLAN.md`.

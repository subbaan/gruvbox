# Sound variety plan

Multi-session plan for making Gruvbox's generated grooves and sounds more
varied. Started 2026-11-06 at 3.0.0-alpha.33, after the desktop UI redesign
(`UI_REDESIGN_PLAN.md`, complete). **Read this before changing generation,
sounds, the 303 voice or effects.** Tick items off, add a session-log line and
update "Next up" at the end of every session.

## The problem (user's words, paraphrased)

Sounds and patterns often sound very similar, especially the random groove
at launch and the 303s. The user uses guitar pedals after real 303s and has a
Roland TB-3 whose built-in sounds range widely; they want that range here.

## Why it happens (audit, alpha.33)

1. **Startup is one fixed recipe.** `desktop.newUI` builds `groove.NewProject()`
   (303, 303, Kick, Snare, ClosedHat, OpenHat at 126 BPM) and runs the same
   generators every time (`Acid`, `Sparse`, `4 Floor`, `2 & 4`, `8ths`,
   `Clear`) with default sounds and no FX. Drum generators are fixed patterns.
2. **The 303 generator is a coin flip per step** (`groove/generate.go`): density
   0.6, notes from one C-minor-ish set at C2, random accents/slides. No motif,
   key or repetition. The much better `PhrasePattern` engine (`groove/phrase.go`:
   styles, scales, roots, motif development, feel) is only used by the Phrase
   drawer.
3. **`RandomiseSound` draws every knob independently and uniformly**
   (`groove/sound.go`), which averages towards the middle of each range.
4. **The 303 voice is narrow** (`synth/tb303.go`, `synth/tb303_filter.go`): 9
   controls, no drive in the voice, fixed +50% accent with no accent-sweep
   build-up, fixed gate, `math.Sin` sub-oscillator in the audio loop.
5. **Effects are generic** (`effects/`): distortion, multimode filter, delay,
   reverb, chorus, bit crusher. Nothing behaves like the pedals people use
   with 303s.

## Phases

### Phase A: variety in generation (no audio-engine changes)

- [x] A.1 Project key: optional `Project.Key` (`root` pitch class 0–11 +
      `scale`), validated, saved with the project. Melodic generators read it so
      parts agree; if absent, the first melodic generation picks one and
      stores it.
- [x] A.2 Melodic actions (303 `Acid`/`Sparse`/`Busy`, Dual and Tom
      `Bassline`) use `PhrasePattern` with the project key, a random phrase style
      and seeds; densities differ per action. `Random` stays the old uniform
      generator. Fall back to the old generator for lengths that are not
      16/32/64.
- [x] A.3 Drum pattern library: families per instrument (kick: four, skip,
      broken, electro, half-time, rolling; snare/clap: backbeat, half-time,
      ghosts, electro; hats: 8ths, 16ths with accents, offbeat, shuffle, sparse
      ticks; open hat: offbeat, pre-beat lifts). Humanised velocity. Drum
      `Random` picks a family + variation instead of uniform noise.
- [x] A.4 Groove styles (`groove/style.go`): Acid, Minimal, Electro, Dub,
      Hypnotic, Broken. Each sets BPM and swing ranges, key/scale choices, phrase
      styles, drum families, sound choices per track (factory presets + small
      jitter) and FX chances. `NewGroove(r)` builds a project from a random
      style; the second 303 complements the first (register, density, phrase
      style, sound).
- [x] A.5 Desktop: launch uses `NewGroove`; `≡ → New ▸ Random groove` makes a
      new one; the status line names the style, tempo and key. Tests keep the
      fixed classic project through a package-level `startupProject` hook set
      in `TestMain`.

*Phase A done in alpha.34. Where it lives:*
- `groove/key.go`: `Key` (`Root` pitch class, `Scale`), `Name`, `BassRoot`
  (G#1–G2), `RandomKey`; `Project.Key` (`json:"key,omitempty"`), cloned and
  validated in `model.go`.
- `groove/generate.go`: `Generate` (random key; kept for callers without a
  project) and `GenerateIn(p, t, slot, action)` (project key, set on first
  use). Internal `generate(t, slot, action, key, r)` takes an explicit
  `*rand.Rand`. `phraseAction` maps Acid/Bassline/Sparse/Busy to
  `PhrasePattern` settings; Tom phrases are folded into C2–C3 (its notes
  transpose from a C2 root; `tom_notes_test` checks this). Drum `Random` uses
  the library.
- `groove/drumlib.go`: `drumFamilies` (16-char templates: `x o g ? .`),
  `DrumFamilies`, `DrumPattern(steps, kind, family, r)`.
- `groove/style.go`: `GrooveStyles` (Acid, Minimal, Electro, Dub, Hypnotic,
  Broken) built from `partRecipe` (actions or drum families, sound spans,
  square/LFO chances, octave, FX chances), `NewGroove`, `NewGrooveInStyle`,
  `GrooveInfo`, `jitterSound` (±6% on every continuous lockable parameter).
  Layout stays 303, 303, Kick, Snare, ClosedHat, OpenHat.
- Desktop: `startupProject` in `desktop/templates.go` (launch and New ▸ Random
  groove via `startRandomGroove` / `replaceProject`); `desktop/main_test.go`
  `TestMain` pins it to the classic layout for UI tests (`randomStartup` keeps
  the real one for `TestRandomGrooveMenu`). Status line shows `GrooveInfo`
  through `Session.LastAction`. Desktop generate buttons and Patterns ▸
  Variation use `GenerateIn`.
- Tests: `groove/drumlib_test.go`, `style_test.go`, `style_render_test.go`
  (renders every style; checks not silent or clipping; logs levels).
  `GRUVBOX_GROOVE_WAVS=<dir> go test ./groove -run TestWriteStyleSamples`
  writes two 4-bar WAVs per style for listening.

Observed while testing: a Busy phrase in Phrygian can sit on just two pitches
(root and ♭2). If the user finds lines too static, raise the minimum
`Movement` in `phraseAction` for Acid/Busy.

### Phase B: pedals (new effect kinds)

- [x] B.1 Overdrive (soft, mid-hump, tone) and Fuzz (two hard stages,
      scooped tone stack) as new kinds; Distortion kept as it was.
- [x] B.2 Envelope filter / auto-wah (follower → band/low-pass, sensitivity,
      range, resonance, decay, up/down).
- [x] B.3 Phaser (4/6/8/12 stages, rate, depth, feedback) and Flanger (short
      modulated delay, feedback, manual).
- [x] B.4 Tape echo (delay with wow/flutter, saturation and darkening repeats)
      and a Compressor (threshold, ratio, attack, release, makeup).
- [x] B.5 Pedalboards: named chain archetypes whose settings vary together;
      used by FX ▸ Pedalboard…, Randomise on an empty chain and the groove
      styles.

*Phase B done in alpha.35. Where it lives:*
- `effects/pedals.go` (Overdrive, Fuzz, helpers `softClip`, `onePole`,
  `glide`, `levelMatch`), `effects/modulation_pedals.go` (`svf`,
  EnvelopeFilter, Phaser, Flanger), `effects/tape_comp.go` (TapeEcho,
  Compressor); tests in `effects/pedals_test.go`. All output fully wet; the
  track's FX mix blends.
- Output level (alpha.54): every track mix goes through the master bus in
  `groove/master.go` (+9 dB make-up, look-ahead limiter at -1 dBFS) for both
  playback and export. Judge new sounds and level rules against that: a
  track that is much louder than its neighbours now drives the limiter
  instead of the old ±0.98 hard clip. `TestGeneratedGroovesComeOutAtANormalLevel`
  expects RMS between -24 and -12 dBFS.
- Drive pedals are level-matched: ~1 s input/output power followers set a
  makeup gain and Level trims ±12 dB around it (`levelMatch`). A fixed output
  calibration was tried first; it only fitted one drive setting and source.
- `groove/fx.go`: kinds (`overdrive`, `fuzz`, `envfilter`, `phaser`,
  `flanger`, `tape`, `compressor`), names, parameters, `EffectKinds` order
  (drives, filters, modulation, time, dynamics), `TempoSynced()` (Delay and
  TapeEcho), `EffectChoices(kind, id)` for stepped selectors (replaces the
  Filter "mode" special cases in validation and the desktop),
  `PhaserStages`, randomise ranges.
- `groove/fx_audio.go`: constructors (phaser/flanger right channel at +¼
  phase), `apply` cases, `updateTempo` for both tempo-synced kinds (tape max
  2 s). TapeEcho's first time setting after construction/reset snaps instead
  of gliding.
- `groove/pedalboard.go`: `Pedalboards` (10 bass boards, 5 drum boards; each
  pedal is an `fxChance` with ranges), `PedalboardNamed`, `RandomPedalboard`
  (suited to track kind), `ApplyPedalboard` (replaces the chain via
  `RemoveEffect` so locks are cleaned), `PedalboardInfo`. `partRecipe` gained
  `board`/`boards` and every style uses boards for its 303s (plus a few drum
  parts).
- Desktop: FX top row has a `Pedalboard…` select (`applyPedalboard`, undo
  label "Pedalboard <name>"); selectors render with a label from
  `EffectChoices`; Delay-only UI now keys off `TempoSynced()`.
- Tests: `groove/pedalboard_test.go` (every board validates and stays within
  ±7 dB of dry; empty-chain randomise; every kind chained renders with zero
  allocations), `desktop/fxcards_test.go` (picker + undo), screenshot
  `pedals`. `GRUVBOX_PEDAL_WAVS=<dir> [GRUVBOX_PEDAL_SOLO=1] go test ./groove
  -run TestWritePedalSamples` writes one Acid groove through every bass board.
  Measured solo: spectral centroid ranges 335 Hz (Falling wah) to 4 kHz (Fuzz
  acid) against 1.5 kHz dry, levels within ±4 dB.

Audio-thread rules (CLAUDE.md): no allocation, locks or I/O in processing;
pre-allocate buffers; smooth parameter changes; add tests like the existing
`effects/*_test.go`.

### Phase C: 303 voice character

- [x] C.1 Accent sweep: accented notes charge a "capacitor" that builds over
      consecutive accents (the classic "wow"); accented notes use a short fixed
      decay.
- [x] C.2 Voice drive/saturation before and after the filter; gate length
      control.
- [~] C.3 Extended modes: pulse width done. Saw/square blend, noise, filter
      mode (LP/HP/BP) and sync not done; optional, pick up only if Phase D
      patches need them.
- [x] C.4 Replace the `math.Sin` sub-oscillator with a fast sine.

*Phase C done in alpha.36 (C.3 partly). Where it lives:*
- `synth/tb303.go`: `SetDrive` (normalised `FastTanh` pre-filter, hotter
  signal into the ladder, then post-filter `FastTanh` with gain `1+6·drive`
  and makeup), `SetAccentSweep` (forwards to the filter), `SetPulseWidth`,
  `NoteOnFor(note, accent, slide, gateSamples)` (gate counter, then a ~4 ms
  release; `NoteOn` = ring). Sub uses `util.FastSine`.
- `synth/tb303_filter.go`: sweep state `charge` (accent: `charge·.7+.4`,
  plain note: `·.5`), `accentEnv` (350 ms per accent), `wow` (30 ms smoothing
  of charge·accentEnv) adds `wow·sweep·(1−cutoff)` to the cutoff. With sweep
  > 0, accented notes get decay `lerp(decay, min(decay,.2), sweep)`.
- `groove`: `Parameters(Acid)` gained `pulse` (.5), `drive` (0), `sweep` (0),
  `gate` (1); all lockable. `voice.acidGate` and `acidGateSamples` (Gate ×
  step length × note length; held when the next step slides).
  `fillParameterDefaults` in `files.go` now fills **any** missing parameter
  for tracks, stored pattern sounds and (in `DecodeSoundPreset`) presets.
  `jitterSound` leaves controls parked at an "off" end alone.
  `RandomiseSound(Acid)` turns each character control on about half the
  time. Styles: Acid (sweep, light drive), Minimal (gate), Electro (short
  gate, drive, narrow pulse), Hypnotic (sweep, gate), Broken (drive, gate).
- Tests: `synth/tb303_test.go` (fast sine, gate, drive, sweep build-up and
  drain, no allocations), `groove/voice_character_test.go` (gate timing,
  migration of old projects/stored sounds/presets, randomise). Listening:
  `GRUVBOX_VOICE_WAVS=<dir> go test ./groove -run TestWriteVoiceSamples`
  (plain, sweep, drive, short gate, narrow square, all together; classic low
  cutoff/high resonance patch). Measured against plain: sweep changes about a
  third of the signal (on accents), drive most of it, gate leaves 63–76% of
  10 ms blocks silent against 7%.
- Also in alpha.36: envelope filter band-pass gain is now `min(2, 10·k)`
  (peaks near Q×1.5 before, which made Squelch wah up to +7 dB), and
  `TestPedalboardsApplyAndKeepLevel` uses a seeded pattern (it was flaky).

### Phase D: sound bank and archetype randomising

- [x] D.1 24 new original 303 patches (28 with the original four) covering
      squelch, driven, modulated, wah, bass/dub and square/lead sounds.
- [x] D.2 Archetype-based `RandomiseSound` for every kind with a bank.
- [x] D.3 Groove styles' 303 parts draw sounds from the bank.

*Phase D done in alpha.37. Where it lives:*
- `groove/acid_bank.go`: `acidBank(add, effect, lfo)` adds the patches after
  the four original Acid factory sounds (indices 0–3 unchanged; tests and the
  desktop preset test rely on them). Each is voice values + an explicit effect
  chain + optional synced LFO (`synced(target, division bars, depth, wave)`).
  `FactorySoundNamed` in `factory_sounds.go`.
- `groove/sound.go`: `RandomiseSound(t) string` wraps
  `RandomiseSoundFrom(t, r) string`. With chance `archetypeChance` (Acid .85,
  Dual .75, drums .6, Sampler 0) it copies a random factory sound's
  parameters and enabled LFOs, jitters ±10% (`jitterSoundBy`), restores
  `randomiseKeeps` (tune everywhere it exists; Dual also osc2_tune and amp
  attack/sustain/release), and gives an empty FX chain the patch's effects.
  Otherwise the old independent draw (now on `r`). Returns the archetype
  name or "".
- Desktop `randomiseSound` previews with a seed on a clone to get the name,
  then performs the same seeded edit labelled "Randomise sound · <name>".
- `groove/style.go`: `partRecipe.patch`/`patches`; `applyRecipe` is now
  `applyRecipePatch` (bank sound via `ApplySound` with FX, `jitterSound`,
  track tune kept) or `applyRecipeSound` (the old spans/square/LFO/boards/
  fx), then `writeRecipeNotes`. Every style lists suitable patches for both
  303s (chance .4–.55).
- Tests: `groove/acid_bank_test.go` (unique names, each patch −12…+9 dB of
  the plain voice on a fixed line, brightness spread ≥3×: measured 5.7×;
  every style patch/board name exists), `presets_test.go` count 54,
  `voice_character_test.go` (randomise keeps tune, many archetypes),
  `desktop/fxcards_test.go` (named status + undo). Listening:
  `GRUVBOX_BANK_WAVS=<dir> go test ./groove -run TestWriteBankSamples`.

## Conventions

- Version: patch-bump `VERSION`, `config/config.go`, `README.md`, `CLAUDE.md`
  for every change; build `go build -o gruvbox`; tests `go test ./...`.
- Generation stays pure and seedable where practical (`math/rand/v2` with an
  explicit `*rand.Rand`) so tests can pin results.
- Desktop UI conventions are in `UI_REDESIGN_PLAN.md` (drawers, `u.live`,
  headless screenshots).

## Follow-ups done in alpha.38

- C.3 finished (except oscillator sync, not wanted): `blend` (Saw/square
  mix, `Oscillator.ProcessBlend` from one phase), `noise` (xorshift32 in
  `TB303.Process`), `filter_mode` (`DiodeLadderFilter.Mode`: 0 y4, 1 BP
  `2y1−2y2`, 2 HP `u−2y1+y2`, before the output saturation). Defaults 0.
  `groove.ParameterChoices(kind, id)` names stepped instrument controls
  (Acid `square` → Saw/Square, `filter_mode`); validation requires integers
  for them; the inspector renders them with `instrumentChoice` (lock-aware
  `paramSelect`). `square` is now labelled "Waveform". Four bank patches use
  the modes (Hollow band, Paper cut, Breath, Half and half); RandomiseSound's
  independent draw turns them on sometimes.
- Drum banks: `groove/drum_bank.go` `drumBank(kind, add, effect)` adds 31
  sounds after each kind's originals (89 factory sounds in all); drum
  archetype chance is .7. `TestDrumBanksAreBalanced` (≥4 per kind, unique
  names, −14…+9 dB against the default).
- Status line: `groove.RandomGroove(r) (Project, info)` names the bank
  sounds the 303s started on ("· 303-1: Fizz acid"); `newGroove` returns them
  per track and `applyRecipe` returns the name. Desktop `startupProject` uses
  it.

## Drum sounds in styles (alpha.39)

Every style's Kick, Snare, ClosedHat and OpenHat recipes list fitting drum
bank sounds (`patch` .3–.45). `applyRecipePatch` keeps the track's tune only
for pitched parts (Acid, Dual, Tom); drums take the sound's tuning.
`RandomGroove`'s status names 303 sounds per track and drum sounds as
"kit: a, b". `TestRandomGrooveNamesBankSounds` checks kits appear.

## Randomise all (alpha.40)

`groove.RandomiseGroove(p, r) string` rebuilds `p.Pattern` for every track:
new style/BPM/swing/key; `clearPatternSlot` (default steps, stored state
dropped), `freshSound` (defaults, default LFOs, chain removed) then the
style recipe via `applyRecipe(..., slot)`. 303s take lead then second;
drums by kind; other kinds `RandomiseSoundFrom` + a fitting action;
samplers keep sound and sample. Status via `grooveStatus`. Seeded and
repeatable: recipe spans and FX params are drawn in sorted key order
(`sortedKeys`), since map order is random. Desktop: top-bar button, ≡ menu
item and Ctrl+R (`u.randomiseGroove`, seeded preview for the undo label).
Tests: `groove/randomise_groove_test.go`, `desktop/keys_test.go`.

When driving the real app with xdotool, find the window with
`xdotool search --pid <pid>` and check it is active before sending keys:
`--name Gruvbox` also matches the user's terminals (titles containing
"gruvbox"). Close it with `kill $PID`, never `pkill -x gruvbox`: the user
may be running their own instance.

## Longer patterns (alpha.41)

`GrooveStyle.bars` (weighted choices: Acid 1,2,2,4; Minimal 1,2,4,4;
Electro 1,1,2,4; Dub 2,4,4; Hypnotic 1,2,4,4; Broken 2,2,4). `newGroove` and
`RandomiseGroove` call `resetPatternSlot(t, slot, bars*16)` for every track
(resolution 16, stored state dropped), so all tracks share the length.
303 lines use 2/4-bar phrases; `DrumPattern` adds a gentle variation at the
end of bar 2 in four-bar patterns (besides the last-bar fill). `GrooveInfo`
adds "· N bars". `TestGroovesComeInOneTwoAndFourBars` checks lengths and
that most multi-bar lead lines change between bars. `TestWriteStyleSamples`
now renders 8 bars with the length in the file name.

## Next up

Nothing planned. Ideas if the user asks: oscillator sync; raising the
minimum phrase `Movement` if lines feel static.

## Session log

- **2026-11-06 (alpha.33):** audit and plan written.
- **2026-11-06 (alpha.34):** Phase A done (A.1–A.5); see the notes under Phase A.
- **2026-11-06 (alpha.35):** Phase B done (B.1–B.5): seven pedal kinds,
  pedalboards, styles use them; see the notes under Phase B.
- **2026-11-06 (alpha.36):** Phase C done (C.3 partly: pulse width only);
  see the notes under Phase C.
- **2026-11-06 (alpha.37):** Phase D done (D.1–D.3); plan complete. See the
  notes under Phase D.
- **2026-11-06 (alpha.38):** follow-ups: extended 303 modes, drum banks,
  bank sounds named in the random-groove status line.
- **2026-11-06 (alpha.39):** styles pick fitting drum bank sounds.
- **2026-11-06 (alpha.40):** Randomise all (in-place fresh groove).
- **2026-11-06 (alpha.41):** grooves in one, two or four bars.

# Changelog

Entries start at 3.0.0-alpha.54. Earlier changes are in the git history and
the session logs in the `*_PLAN.md` files.

## 3.0.0-alpha.62

**The Dual synth and the SH-101 now do different jobs.**

- **Dual synth, new controls** (all off by default, so existing sounds and
  projects are unchanged):
  - **Oscillator 1 level**, so you can hear oscillator 2 alone.
  - **Oscillator 2 sync**: oscillator 2 restarts with oscillator 1. Raise its
    interval, or **Osc 2 sweep** (the filter envelope bends its pitch up to
    two octaves), for the tearing sync lead.
  - **FM amount**: oscillator 2 bends oscillator 1's phase. Sines give bells
    and knocking basses.
  - **Ring mod**: the two oscillators multiplied, for metallic tones.
  - **Unison**: two detuned copies of oscillator 1, up to 25 cents either
    side.
  - New LFO targets: **Oscillator 2 pitch**, **FM amount** and **Ring mod**.
  - They are in the Sound tab's new **Sync, FM and ring mod** section
    (unison and oscillator 1 level are under Oscillators).
- **Nine new Dual sounds:** Sync lead, Sync sweep, FM bell, FM bass, Ring
  clang, Unison lead, Fifth stab, Octave stab and Minor stack. Randomise
  sound now sometimes turns on sync, FM, ring mod or unison.
- **New generators:** **Lead** (a slow melody of held notes that slide into
  each other) and **Stabs** (short offbeat hits on each bar's chord root) on
  the Dual; **Arp** (the bar's chord, up, down or shuffled) and **Sequence**
  (a short figure of 3–8 notes that cycles across the bar line) on the
  SH-101.
- **In grooves**, the SH-101 now plays arpeggios, sequences and basslines,
  and no longer copies the Dual's parts. The Dual plays held melodies (a new
  role), interval stabs, pads, counter lines and layers. Projects that
  already give an SH-101 a Dual role keep working as before.
- **Fix:** when a groove or Randomise sound varied a Dual or SH-101 sound, an
  envelope time could jump by a large share of its whole range. A 0.1 s
  decay often ended up as a 2 ms click, and an attack could grow by a quarter
  of a second. Envelope times now vary by a ratio (about ±27%).
- The README is now a user guide. Developer notes moved to
  `docs/DEVELOPMENT.md` and the legacy terminal UI's keys to `docs/TUI.md`.
- The app ID is now `io.github.subbaan.gruvbox`. Gruvbox keeps nothing under
  the app ID, so no settings move.

## 3.0.0-alpha.61

- Pattern exports can wrap the effect tail onto the start (**Effect tail:
  Loop**, now the default for patterns and single-track pattern renders).
  The file is exactly one loop long and loops seamlessly: the loop plays
  silently first so reverb and delay from the previous pass are already in
  its start. Song exports keep **Add tail at end**; **No tail** is a choice
  for both.

## 3.0.0-alpha.60

- The track menu's render item now says it renders that track alone:
  **Render this track to WAV ▸ <track> only · selected pattern… / full
  song…**, and the export window says the other tracks are left out.

## 3.0.0-alpha.59

- The save dialog selects the file name without its extension, so typing
  replaces the name and keeps `.gruvbox3`.
- A groove with no file yet is named after its style, key and tempo (for
  example `acid-a-minor-128bpm.gruvbox3`) instead of `untitled.gruvbox3`.
  WAV exports use the same name.
- After Randomise, Grow into a track, Jam or a groove history restore,
  Ctrl+S asks before overwriting the file you saved before: Save as new…,
  Overwrite or Cancel.
- Right-click a track name → **Render to WAV** exports that track alone, for
  the selected pattern or the full song.
- The piano roll marks each quarter note (lit column, brighter step number,
  a line before it) and each bar (a stronger line).

## 3.0.0-alpha.58

- **SH-101**, a new mono synth track modelled on the Roland SH-101. Add it in
  **+ Track**.
  - **VCO and mixer:** one oscillator with a 16'/8'/4'/2' range switch.
    Saw, pulse, sub-oscillator and noise each have a level and play
    together. The sub is locked to the oscillator. **Pulse width mod** sets
    the width by hand, or lets the LFO or the envelope sweep it.
    **Pitch LFO** adds vibrato, up to a siren.
  - **VCF:** a 24 dB four-pole low-pass that self-oscillates at full
    resonance, with envelope, LFO and key-tracking amounts.
  - **Envelope and VCA:** one ADSR shapes the filter and the level. **Amp**
    follows the envelope or just the gate. **Envelope trigger:** Gate holds
    through slides, Gate + trigger restarts on every note, LFO repeats it
    while a note is held (trills).
  - **LFO** with triangle, square, random and noise waves; **portamento**
    Off, On or Auto (slide steps only); **accent** opens the filter.
  - Eleven sounds: 101 bass, Sub pressure, Acid 101, Rubber pluck, PWM lead,
    Squelch stab, Trill, Siren, Noise zap, Hollow organ and Sample and hold.
  - New grooves sometimes use an SH-101 in the Dual's place.

## 3.0.0-alpha.57

- **Answering the bass:** counter lines, stabs and second 303 lines play in
  the bass line's gaps instead of on top of it. A note that would start with
  a bass note moves a step later or earlier into a rest; a few shared hits
  stay, mostly on a bar's first beat. Pads still ring over the bass and
  layers still double it.

## 3.0.0-alpha.56

- **Fit groove:** each groove remembers its style and each synth its part in
  it (lead, bass, second line, pad, counter line, stabs or layer).
  **+ Track** sets a new track up the way the style would (loop length,
  sound, level, effects), and **G** writes a part that suits the groove.
  Anything that is not the bass sits at least an octave above it. Jam brings
  synths in, and rephrases them, in their role too.

## 3.0.0-alpha.55

- **≡ → New groove** (**Ctrl+N**) starts again with a fresh random groove,
  like launching the app. The fixed layouts moved to **≡ → New from
  template ▸**.

## 3.0.0-alpha.54

- **Master bus:** playback and WAV export go through +9 dB of make-up gain
  and a look-ahead limiter at -1 dBFS instead of a hard clip. Generated
  grooves come out about 9 dB louder without clipping. Exported loops still
  start on the downbeat.
- Example loops in the README.

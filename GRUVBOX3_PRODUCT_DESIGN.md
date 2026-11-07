# Gruvbox 3. Product and Interaction Design

**Document version:** 0.1.1  
**Project target:** Gruvbox 3.0.0  
**Status:** Foundational design baseline  
**Source reference:** Gruvbox 2.16.9

## 1. Purpose

Gruvbox 3 is a performance-oriented, generative groovebox for rapidly creating beats, basslines, melodic ideas and short songs, then editing those ideas in enough depth to make them personal.

The defining workflow is:

> **Generate quickly. Hear something interesting. Keep it. Edit it deeply. Turn it into a little song.**

Gruvbox should feel closer to sitting down with a hardware groovebox than opening a general-purpose DAW. It should be quick enough to rescue a DJ set that needs another rhythm immediately, playful enough for aimless experimentation, and deep enough to support a relaxed half-hour of refining a happy accident.

The application may grow substantially beyond the current terminal version, but speed, playfulness and direct musical interaction remain more important than DAW-style comprehensiveness.

---

## 2. Product principles

### 2.1 Silence to groove in seconds

A user should be able to create a useful rhythm from an empty or default project in a handful of actions.

Examples:

- Select Kick, click **4 Floor**.
- Select Closed Hat, click **Offbeat** or **16ths**.
- Select Snare, click **2 & 4**.
- Select a 303 or synth track, click **Acid**, **Bassline** or **Random**.
- Press Play.

The target is a credible starting groove in approximately five to ten deliberate actions, without opening modal dialogs or navigating deep menus.

### 2.2 Randomness is a creative partner

Generation and randomisation are core Gruvbox workflows, not utility commands hidden under a Tools menu.

The application should distinguish between:

- **Generate**. Create a new musically coherent part.
- **Mutate**. Preserve the identity of the existing part while changing selected aspects.
- **Variation**. Create a related sibling pattern suitable for arrangement.
- **Fill**. Add temporary or committed embellishment.
- **Random**. Allow broader, less predictable experimentation.

All destructive generative operations must create an immediate undo/history point. A useful happy accident must be easy to keep, compare or restore.

### 2.3 Editing depth remains core

Fast generation must never replace manual composition.

Users must be able to take a generated result and edit it in detail. Gruvbox 3 retains and expands the current concepts of:

- individual step entry
- note entry
- velocity
- accent
- slide or glide
- probability
- playback conditions
- per-step parameter locks
- variable pattern length
- variable resolution
- polymetric track lengths
- transposition
- pattern duplication and variation
- piano-roll editing for melodic material
- direct parameter editing
- sound design
- effects
- song arrangement
- sample trimming and editing

The fast layer should get the user to an idea. The editing layer is where that idea becomes theirs.

### 2.4 Gruvbox is an instrument, not a miniature DAW

Gruvbox should provide substantial musical control while avoiding features that primarily serve linear recording, studio engineering or unrestricted routing.

The design should favour:

- patterns over an infinite linear timeline
- tracks as musical instruments
- direct controls over configuration pages
- musical generators over scripting systems
- a constrained, understandable signal path
- performance gestures over detailed automation-lane management
- song scenes and pattern arrangement over general audio-region editing

A feature should earn its place by improving groove creation, sound design, live use, sampling or arrangement.

### 2.5 Mouse and keyboard are both first-class

A GUI should improve discoverability, direct manipulation, waveform editing and plugin use without losing the speed of the TUI.

Every frequently used action should have a strong keyboard path. Core workflows should be practical with mouse only, keyboard only, or a mixture of both.

### 2.6 The default setup is a template, not a limitation

Gruvbox 2 was built around fixed instruments. Gruvbox 3 must use a dynamic track model.

A **Classic Gruvbox** new-project template may provide a familiar starting setup such as:

- TB-303 1
- TB-303 2
- Kick
- Snare
- Closed Hat
- Open Hat
- Rimshot
- Crash
- Tom
- Synth
- Sample 1
- Sample 2

These are simply initial tracks. They do not define the engine's maximums or project schema.

Users can add, remove, duplicate and reorder tracks. A project may contain one sample track, twelve sample tracks, several plugin instruments, multiple synths, or a very small minimal setup.

---

## 3. Core musical model

### 3.1 Dynamic tracks

A project contains an ordered collection of tracks.

Each track has a stable ID, display name, colour, track type, sequencer state, instrument, mixer state, effect chain and optional track-specific data.

Conceptually:

```text
Track
  ID
  Name
  Colour
  Type
  Instrument
  Patterns
  Mixer
  Effects
  GeneratorProfile
  PerformanceMacros
```

Track types may include:

- built-in melodic instrument
- built-in drum instrument
- sampler
- plugin instrument
- MIDI/event generator plus instrument

The UI and project format must never rely on hard-coded channel numbers such as "Sampler 1 is channel 7".

### 3.2 Track capabilities

Each instrument exposes capabilities to the rest of Gruvbox.

Examples:

```text
TB-303
  Notes          yes
  Velocity       yes
  Accent         yes
  Slide          yes
  P-Locks        yes
  Generator      acid/bass

Kick
  Notes          optional/limited
  Velocity       yes
  Accent         yes
  Slide          no
  P-Locks        yes
  Generator      kick rhythms

Sampler
  Notes          mode dependent
  Velocity       yes
  Accent         optional
  Slide          mode dependent
  P-Locks        yes
  Generator      rhythm/sample variation
```

The UI derives available editing controls from capability information instead of maintaining separate hard-coded screens for every instrument.

### 3.3 Parameter definitions

Instrument and effect parameters should use stable parameter definitions containing at least:

- stable ID
- display name
- type
- minimum and maximum
- default
- coarse and fine increments
- display formatter
- whether the parameter can be p-locked
- whether it can be exposed as a performance macro
- whether randomisation is permitted and its musical randomisation range

This removes the need for separate switch statements throughout the UI, p-lock system, project serializer and randomisation system.

---

## 4. Main application layout

Gruvbox 3 should use a dense, flat, dark GUI. It should retain some of the clarity and economy of a terminal application while taking advantage of graphical controls.

The primary layout is persistent:

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│ GRUVBOX     ▶ 126 BPM    SWING 54%    PAT 03    BAR 17:1    SONG   REC      │
├──────────────────┬──────────────────────────────────────┬────────────────────┤
│ TRACK LIST       │ MAIN EDITOR                          │ TRACK INSPECTOR    │
│                  │                                      │                    │
│ 303-1            │ sequencer / piano roll / locks       │ SOUND              │
│ 303-2            │                                      │ SEQ                │
│ Kick             │                                      │ LOCK               │
│ Snare            │                                      │ FX                 │
│ Hats             │                                      │ MIX                │
│ Sample           │                                      │                    │
│ Plugin           │                                      │ parameters         │
│ ...              │                                      │                    │
├──────────────────┴──────────────────────────────────────┴────────────────────┤
│ CONTEXT ACTION DECK:  GENERATE  MUTATE  VARIATION  FILL  RANDOM  CLEAR      │
├──────────────────────────────────────────────────────────────────────────────┤
│ TRACK      PATTERN      SONG      SAMPLE      MIXER      BROWSER      TOOLS  │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4.1 Top transport

The top bar should remain compact and persistent.

It should include:

- Play/Stop
- Record when applicable
- BPM
- swing
- time signature if supported
- current pattern
- current song position
- metronome if later required
- song/performance status
- project dirty indicator

Transport controls should never disappear when changing editors.

### 4.2 Track list

The left track list is the stable anchor of the project.

Each row should show only information valuable at a glance:

- colour strip
- track name
- mute
- solo
- optional compact level activity
- selected state

Track creation uses a fast **Add Track** control with categories:

```text
Built-in
  TB-303
  Drum Synth
  Synth
  Sampler

Plugins
  Recently Used
  Instruments...
  Generators...
```

Track duplication should be trivial, particularly for sampler and plugin tracks.

### 4.3 Main editor

The centre of the application is the currently selected musical editor.

For most work it displays either:

- step sequencer
- piano roll
- parameter-lock lane/editor
- sample waveform/editor
- song arrangement

The editor should preserve context when possible. Selecting another track should change the data being edited rather than constantly moving the user to another screen.

### 4.4 Track inspector

The right inspector follows the selected track.

Primary tabs:

```text
SOUND | SEQ | LOCK | FX | MIX
```

For plugin instruments:

```text
SOUND | SEQ | LOCK | PLUGIN | FX | MIX
```

`SOUND` should expose a compact set of important controls. Less frequently used controls can expand beneath them.

For external plugins, selected parameters can be mapped to Gruvbox's own inspector while **Open Plugin** launches the full native plugin editor.

### 4.5 Context Action Deck

The action strip beneath the main editor is a defining Gruvbox element.

Its contents change with the selected track and current editor.

Kick example:

```text
[4 FLOOR] [HOUSE] [BROKEN] [SYNCOPATED] [EUCLIDEAN] [MUTATE] [RANDOM]
```

Closed hat example:

```text
[8THS] [16THS] [OFFBEAT] [SHUFFLE] [SPARSE] [MUTATE] [RANDOM]
```

Snare example:

```text
[2 & 4] [HALFTIME] [BREAKBEAT] [GHOSTS] [FILL] [MUTATE] [RANDOM]
```

303 example:

```text
[ACID] [SPARSE] [BUSY] [SYNCOPATED] [MUTATE] [VARIATION] [RANDOM]
```

The quick buttons use musically sensible defaults. A secondary gesture or small expander opens deeper generator controls.

---

## 5. Sequencing and detailed editing

### 5.1 Step sequencer

The step sequencer remains the fastest way to understand and alter a groove.

Each track can have its own:

- pattern length
- step resolution
- playback position
- pattern content

This preserves polymetric behaviour.

The sequencer should support horizontal expansion beyond 16 steps without making ordinary 16-step work feel oversized.

Suggested visual states:

- inactive
- active
- accented
- slide/glide
- probability/condition present
- p-lock present
- current playback step
- selected step

The grid should communicate these states through a restrained combination of fill, small glyphs and track colour.

### 5.2 Step Detail editor

Selecting a step exposes detailed editing without leaving the main workflow.

For a melodic step:

```text
STEP 09

Note          C#3
Velocity      91
Accent        On
Slide         On
Probability   78%
Condition     Every 3rd

Parameter Locks
Cutoff        72%
Resonance     84%

[Clear Locks] [Copy Step] [Paste Step]
```

Controls irrelevant to the selected track capability remain hidden.

### 5.3 Piano roll

Piano roll is a core editing surface, not an optional afterthought.

Melodic tracks should be switchable between **Steps** and **Piano Roll** while editing the same underlying pattern.

The piano roll should support:

- click or keyboard note entry
- drag note vertically to transpose
- move note horizontally
- note length where the instrument supports held notes
- velocity editing
- slide/glide indication
- accent
- probability
- condition
- selected p-lock indication
- scale highlighting
- octave navigation
- audition on click or keyboard entry
- multi-selection
- transpose selection
- duplicate
- delete
- quantised placement

For 303-style sequencing, slides should be particularly easy to enter and visually obvious.

The piano roll must remain lighter than a DAW MIDI editor. Its purpose is fast groovebox composition.

### 5.4 Note entry

Keyboard-as-piano entry from Gruvbox 2 should remain available.

A user should be able to arm note entry, play a computer-keyboard note and have the cursor advance. Mouse entry and external MIDI input should eventually coexist with it.

### 5.5 Parameter locks

Per-step locks remain one of the strongest ways to turn a repetitive pattern into something expressive.

Gruvbox should support both:

- step-centric editing, select a step and edit all locks on it
- parameter-centric editing, select a parameter and see/edit its values across the pattern

A compact lock lane beneath the main sequencer can provide Elektron-like visibility without turning into full DAW automation.

---

## 6. Generation, mutation and variation

### 6.1 Musical generators

Generators should operate on musical concepts rather than pure random values.

They may consider:

- track role
- scale/key
- density
- syncopation
- current pattern length
- neighbouring notes
- existing accents/slides
- current groove/swing
- other tracks where useful

### 6.2 Context-aware generation

Generators should eventually be able to react to the project around them.

Examples:

- bassline generation can reinforce or deliberately avoid kick positions
- ghost snares can be placed relative to kick/snare structure
- hats can complement existing percussion density
- a second acid line can avoid duplicating the first
- fills can understand phrase boundaries
- variation generation can preserve recognisable anchors

This should feel musical and predictable enough to trust. It should not require the user to understand an "AI" system.

### 6.3 Generator depth controls

Quick generation uses good defaults. Expanded controls may include:

```text
Density       65%
Syncopation   40%
Range         1 octave
Variation     35%

Affect:
[x] Notes
[x] Rhythm
[x] Accent
[x] Slide
[ ] P-Locks
```

### 6.4 Non-destructive creative history

Every Generate, Mutate, Variation and broad Random operation should create a recoverable history state.

The user should be able to:

- Undo immediately
- Redo
- compare recent generated versions
- optionally promote a favourite variation into another pattern slot

A later enhancement could provide a small **Ideas** history containing the last several generated states for the current pattern.

---

## 7. Sampler tracks

### 7.1 Unlimited-by-design sampler tracks

Gruvbox 3 must not contain fixed `Sampler1` and `Sampler2` concepts in its core architecture.

Sampler tracks are ordinary dynamic tracks. The practical number of sampler tracks is governed by system resources and sensible UI limits, not a project schema constant.

A user might create:

```text
Kick Sample
Vocal Chop
Break Loop
FX Hit
Texture
Bass One-shot
Percussion Loop
```

Each remains independently sequenceable, mixable, effectable and arrangeable.

### 7.2 Sampler workflow

A sampler track should make this path quick:

```text
Add Sampler Track
    ↓
Drop/select/record sample
    ↓
Trim
    ↓
Choose playback behaviour
    ↓
Sequence
```

### 7.3 Core sample editor

The built-in editor should retain the useful Gruvbox 2 concepts and improve them visually:

- waveform
- start marker
- end marker
- loop region
- audition
- trim/crop
- normalize
- reverse
- fade in/out
- gain
- zoom
- zero-crossing assistance where useful

Editing should be reversible until explicitly committed where practical.

### 7.4 Playback modes

Initial sampler modes should stay focused:

- one-shot
- gate
- loop
- pitched melodic playback

Slicing is a strong future addition, especially for breaks, but does not need to block the first Gruvbox 3 milestone.

---

## 8. Plugins

### 8.1 Goal

Plugin support should let external instruments and effects participate in the Gruvbox workflow rather than turning Gruvbox into a generic plugin rack.

The guiding idea is:

> **Anything suitable can become a Gruvbox instrument.**

### 8.2 Initial plugin formats

Planned priority:

1. VST3
2. CLAP
3. LV2 when the host layer is stable

VST2 is not a design target.

### 8.3 Plugin roles

Gruvbox should understand three plugin roles:

- instrument
- audio effect
- MIDI/event generator

A plugin instrument receives Gruvbox sequencing and participates in the normal mixer/effect chain.

A generator may feed notes/events into either a built-in or plugin instrument.

### 8.4 Constrained routing

Initial routing should remain understandable:

```text
Optional Event Generator
        ↓
Instrument
        ↓
Serial Track FX
        ↓
Track Mixer
        ↓
Master
```

This provides substantial power while avoiding an unrestricted DAW routing graph.

### 8.5 Plugin parameters

Plugins may expose many parameters. Gruvbox should let the user map a smaller set of useful parameters into its own inspector and performance controls.

For example:

```text
Cutoff
Resonance
Drive
Envelope
Delay Mix
Macro 1
Macro 2
```

Mapped parameters become eligible for Gruvbox p-locks and controlled randomisation where technically safe.

### 8.6 Native plugin windows

The first implementation should open third-party plugin editors in their own windows rather than making embedded plugin GUI hosting a requirement for Gruvbox 3.0.

### 8.7 Plugin isolation

The architecture should allow plugins to move out of the main Gruvbox process. A plugin crash should ultimately be recoverable as a bypassed or reloadable track/effect rather than terminating Gruvbox.

This can be phased after basic hosting works, but the control/audio boundary should not make isolation impossible.

---

## 9. Effects and mixer

### 9.1 Unified effect chain

Built-in and plugin effects should appear in the same serial track chain.

Example:

```text
1  Gruvbox Distortion
2  External Delay
3  Gruvbox Filter
4  External Reverb
```

Effects can be:

- added
- removed
- bypassed
- reordered
- edited
- p-locked where appropriate

### 9.2 Mixer

The mixer view should scale horizontally with dynamic tracks.

Each track needs at minimum:

- level
- pan
- mute
- solo
- compact meter

Advanced sends or buses should only be introduced when there is a concrete musical use that justifies the added routing complexity.

---

## 10. Song mode and arrangement

Gruvbox should retain pattern-based song construction.

The goal is to let the user turn a promising loop into a small complete piece without entering a DAW workflow.

Song mode should support:

- ordered sections or song steps
- pattern selection
- repeat count or length
- track mute states where useful
- pattern variations
- duplication
- reordering
- transition/fill behaviour later

Suggested presentation:

```text
01  Intro        Pattern 01    x4
02  Beat In      Pattern 02    x8
03  Main A       Pattern 03    x16
04  Variation    Pattern 04    x8
05  Breakdown    Pattern 05    x8
06  Main B       Pattern 06    x16
07  Outro        Pattern 07    x8
```

The arrangement may also have a compact block/timeline view, but the primary mental model remains a list of musical sections and patterns.

---

## 11. Performance mode

A dedicated Performance view can make Gruvbox useful when immediate interaction matters more than detailed editing.

It should present large, meaningful musical actions rather than a giant step grid.

Example:

```text
┌────────────────┬────────────────┬────────────────┬────────────────┐
│ KICK           │ HATS           │ SNARE          │ ACID           │
│                │                │                │                │
│ [4 FLOOR]      │ [16THS]        │ [2 & 4]        │ [ACID]         │
│ [BREAK]        │ [OFFBEAT]      │ [BREAK]        │ [MUTATE]       │
│ [MUTATE]       │ [SHUFFLE]      │ [FILL]         │ [RANDOM]       │
│                │                │                │                │
│ LEVEL          │ LEVEL          │ LEVEL          │ CUTOFF         │
└────────────────┴────────────────┴────────────────┴────────────────┘

[BREAKDOWN] [BUILD] [FILL] [DROP] [MUTE GROUP] [MUTATE SELECTED]
```

Performance mode should eventually support user-selected tracks and macros rather than assuming a fixed drum/303 configuration.

---

## 12. Appearance

### 12.1 Visual direction

Theme support is a foundational requirement. All Gruvbox-owned UI surfaces must use a shared theme system from the first GUI milestone.

Ship a restrained warm dark default: charcoal backgrounds, bone text, amber selection, and muted olive, rust and slate accents. Gruvbox Dark may be offered as another bundled theme. The mockups in `sources/` are flexible references for density and interaction, not mandatory layouts or palettes.

The desired feel is:

- dark
- flat
- compact
- warm rather than clinical
- terminal-influenced
- hardware-like
- highly legible
- restrained

Avoid excessive gradients, glossy knobs, thick panel borders and decorative pseudo-hardware.

### 12.2 Colour use

Colour should primarily communicate information.

Use it for:

- track identity
- active steps
- accents/slides/locks
- selection
- transport state
- warnings
- meters

Large background areas remain quiet and dark.

The interface should remain understandable without every track becoming a rainbow. Track colours should be somewhat muted until active/selected.

### 12.3 Typography

Use a readable monospaced or semi-monospaced family for sequencer data, values and technical labels. General UI labels may use the same family if it remains comfortable at small sizes.

The visual language should retain some lineage from the TUI without pretending to be a terminal.

### 12.4 Density

Gruvbox should be comfortable at 1440p and usable at smaller desktop resolutions.

Controls should be compact enough that the user can see the groove, instrument controls and generative actions together.

Whitespace should improve grouping rather than consume working area.

### 12.5 Base16 themes and semantic colours

Support Base16 YAML schemes, including both the current Tinted Theming format and legacy flat `base00` through `base0F` schemes. Accept dark and light schemes; the default appearance is dark, but component styling must work with either.

Map scheme colours into named UI roles in one central resolver. Components consume roles rather than embedding hex colours or making their own Base16 mappings.

Roles must cover:

- application, panel, input and alternate-row backgrounds
- primary, secondary and disabled text
- borders, focus, selection and selected text
- buttons, tabs, sliders, knobs and their hover/pressed/disabled states
- inactive and active steps, playback position, accent, slide and p-lock indicators
- track colour slots, piano-roll notes, scale shading and velocity lanes
- waveforms, sample markers and loop/trim selections
- mixer meters, clipping, transport, recording, warnings and errors

Use `base00` through `base07` for surface and text roles and `base08` through `base0F` for accents and status roles, with explicitly defined foreground/background pairs. Check important text and focus contrast against their actual surfaces. Derived colours must come from the selected scheme, not fixed dark-theme assumptions.

Track identity should normally store a stable logical colour slot, resolved through the active theme. Repeated colours are acceptable when there are more tracks than accent slots; track names and selection remain clear. Imported or user-chosen literal colours may be preserved as explicit overrides.

Playback, selection, accent, slide, locks and warnings also need glyphs, outlines or labels so colour alone does not carry their meaning. Third-party native plugin editors retain their own appearance; Gruvbox-owned plugin controls use the application theme.

### 12.6 Configuration and theme downloads

Provide **Conf / Settings → Appearance** with:

```text
Theme             [Warm Dark                 v]

[Preview] [Apply] [Cancel]
[Download themes] [Import theme] [Reset to default]
```

**Download themes** opens a searchable catalogue from GitHub. The default source is the Base16 schemes in [tinted-theming/schemes](https://github.com/tinted-theming/schemes). Show each theme's name, author, light/dark variant and palette preview, with download and update actions. Catalogue refresh and downloads run in the background with progress, cancellation and useful error messages, without blocking editing or audio.

Download palette data over HTTPS, validate the complete sixteen-colour scheme, and write it atomically into a local theme library. Themes are data only. A malformed file or failed update must leave the last valid copy and active theme intact. Preserve source identity and revision metadata so updates are traceable; refreshes must not overwrite user imports or custom edits.

Cache the catalogue and installed themes for offline use. Include bundled themes so first launch never needs a network connection. Allow local YAML imports as well as GitHub downloads.

Preview updates all open Gruvbox surfaces immediately. Apply persists the chosen theme in application configuration; Cancel restores the previous theme. Switching themes must preserve musical state and playback and require no restart. The selected theme is a user preference, independent of project files.

### 12.7 Theme acceptance criteria

- Every Gruvbox-owned editor, inspector, dialog, browser and performance surface responds to theme changes.
- A bundled dark theme and an imported light Base16 theme both keep text, selection and playback indicators legible.
- Theme download, validation and application never enter the audio callback.
- Installed themes remain usable offline; an invalid import or failed download keeps the current theme usable.
- Preview/Cancel restores the previous appearance, and an applied theme survives restart.

---

## 13. Project format

Gruvbox 3 requires a new versioned, track-oriented project format.

Conceptually:

```text
Project
  FormatVersion
  Metadata
  Tempo
  Swing
  Tracks[]
    ID
    Name
    Type
    InstrumentState
    Patterns[]
    MixerState
    Effects[]
    GeneratorState
  Song
```

The format must avoid fixed arrays for instrument slots, mixer channels or sample tracks.

New instruments and track types should normally serialize as additional track entries rather than requiring top-level schema expansion.

A Gruvbox 2 importer is out of scope. The sole current user does not need legacy project conversion.

---

## 14. Audio and application architecture

### 14.1 Strong UI/audio boundary

The GUI must not directly mutate real-time synth, effect or sequencer objects while the audio thread is using them.

UI actions become commands such as:

```text
SetParameter(trackID, cutoff, 0.72)
ToggleStep(trackID, patternID, step)
SetNote(trackID, patternID, step, C3)
SetSlide(trackID, patternID, step, true)
SetPatternLength(trackID, 13)
LoadSample(trackID, path)
AddEffect(trackID, effectType)
```

Audio-critical changes are applied safely at controlled boundaries.

### 14.2 Real-time requirements

The audio callback must avoid:

- heap allocation
- filesystem I/O
- GUI calls
- project serialization
- sample decoding
- blocking locks

Sampler playback and voice processing should use preallocated or reusable buffers.

### 14.3 Background work

The following belong outside the real-time audio callback:

- sample loading/decoding
- waveform analysis
- plugin scanning
- project save/load
- sample normalization/rendering
- preset browsing

---

## 15. Undo, history and safety

Gruvbox 3 should have a proper edit history from the start.

Undoable operations include at least:

- step edits
- note edits
- parameter changes where practical
- generator results
- randomisation
- mutation
- pattern clear
- pattern paste
- track add/remove/reorder
- effect changes
- sample trim operations
- song arrangement edits

Random experimentation becomes dramatically more enjoyable when every risky action is reversible.

---

## 16. New-project experience

On startup or New Project:

```text
NEW PROJECT

[Classic Gruvbox]
2×303, drum synths, synth, 2 sampler tracks

[Beat Box]
Drums + sampler-focused setup

[Acid Box]
303-focused setup

[Blank]
Start with no tracks
```

Templates are convenience only. Any track can subsequently be added or removed.

The Classic template preserves the character and immediate familiarity of Gruvbox 2 without carrying its fixed-track architecture forward.

---

## 17. Suggested keyboard philosophy

Exact bindings can be refined during implementation, but the common actions should remain extremely fast.

Candidate defaults:

```text
Space           Play/Stop
R               Randomise selected context
Shift+R         Mutate selected context
G               Open/trigger generator
F               Fill
M               Mute selected track
S               Solo selected track
[ / ]           Previous/next pattern
Ctrl+D          Duplicate pattern/selection
Ctrl+Z          Undo
Ctrl+Shift+Z    Redo
Delete          Clear selected step/note/item
Enter           Edit/open selected item
1..9            Fast track selection where practical
```

Context-sensitive shortcuts should be discoverable in a compact footer/help overlay rather than requiring memorisation from documentation.

---

## 18. Scope boundaries

### Gruvbox 3 should do well

- step sequencing
- piano-roll melodic editing
- acid sequencing
- drums
- sampling
- generative patterns
- mutation and variation
- p-locks
- probability and conditions
- polymeters
- plugin instruments/effects
- compact mixing
- pattern-based song arrangement
- live/performance interaction

### Gruvbox 3 does not initially need

- unlimited linear multitrack audio recording
- comping
- detailed audio-region timeline editing
- video
- score notation
- arbitrary routing matrices
- large mastering toolchains
- DAW-style automation lane systems
- complex bus/group architecture

These boundaries should be revisited only when a real Gruvbox workflow is blocked by their absence.

---

## 19. Implementation direction

A sensible implementation sequence is:

### Phase 1. Core model

- application configuration, Base16 theme loader and shared semantic colour definitions
- dynamic project tracks
- track capability model
- parameter definitions
- versioned project format
- undo/history model
- command boundary between UI and engine

### Phase 2. Audio foundation

- port useful Gruvbox 2 DSP
- remove real-time allocations
- dynamic mixer
- dynamic effect chains
- safe sequencer/audio communication

### Phase 3. Main GUI shell

- transport
- dynamic track list
- central sequencer
- inspector
- Context Action Deck
- keyboard navigation
- theme-driven visual system across all GUI components
- Conf / Settings appearance panel, live theme preview and GitHub theme downloads

### Phase 4. Editing depth

- step details
- note entry
- piano roll
- accent/slide
- probability/conditions
- p-lock editing
- variable lengths/resolutions

### Phase 5. Generative workflow

- generators by track role
- mutate
- variation
- fills
- controlled randomisation
- generation history/undo

### Phase 6. Sampling

- dynamic sampler tracks
- browser/load/record path
- waveform editor
- trim/loop/reverse/normalize/fades
- one-shot/gate/loop/pitched modes

### Phase 7. Song and performance

- pattern arrangement
- section workflow
- performance view
- live quantised actions

### Phase 8. Plugins

- VST3/CLAP host layer
- plugin scanning/database
- instrument plugins
- effect plugins
- parameter mapping
- plugin state persistence
- separate plugin editor windows
- event generator support
- later LV2 and process isolation

### Phase 9. Migration and polish

- Broader classic instrument templates
- templates
- presets
- accessibility/keyboard audit
- performance profiling
- crash recovery
- documentation

---

## 20. Gruvbox 3 identity test

When considering a new feature, ask:

1. Does it help the user create a groove faster?
2. Does it make experimentation more rewarding?
3. Does it help turn a generated idea into something personal?
4. Does it improve live interaction?
5. Does it deepen sound design without adding unnecessary workflow friction?
6. Can it fit the groovebox mental model without pulling the application toward a general DAW?

A Gruvbox session should comfortably support both extremes:

```text
"I need kick, hats, snare and an acid line right now."
```

and:

```text
"That random pattern is strangely good. I am going to sit here for half an hour,
move notes around, change slides, tweak locks, edit the sound, make variations,
and turn it into a little song."
```

Both are the same product. The speed creates the idea. The editing depth creates attachment to it.

---

## 21. Current design decisions

The following are considered baseline decisions for Gruvbox 3 unless implementation evidence gives a strong reason to revisit them:

- GUI application rather than TUI-only application.
- Keep strong keyboard operation and terminal-inspired density.
- Dynamic number of tracks.
- Dynamic sampler tracks. No fixed two-sampler limit.
- Default instrument setup implemented as templates.
- Track-oriented, versioned project format.
- Step sequencer and piano roll both remain first-class editors.
- Manual notes, accents, slides, probability, conditions and p-locks remain core.
- Generation, mutation, variation and randomisation are first-class UI actions.
- Proper undo/history is required for creative experimentation.
- Built-in instruments remain important even with plugins.
- VST3 and CLAP are the first plugin-host targets.
- Plugins participate in normal Gruvbox sequencing, effects and mixing.
- Plugin hosting must not dictate a DAW-like interface.
- Built-in and plugin effects share a unified track effect chain.
- Song mode remains pattern-oriented.
- Performance mode is a dedicated future-facing workflow, not simply a larger sequencer.
- Base16 theme support and shared semantic colours are required from the first GUI milestone.
- Conf / Settings provides GitHub theme downloads, local imports and persistent theme selection.
- The bundled default uses a restrained warm palette; mockup colours and layouts are flexible references.
- The audio thread has a strict real-time-safe boundary.


# Legacy terminal interface

`./gruvbox --tui` runs the original terminal groovebox. It has its own,
older feature set and project format (`.gruvbox` JSON files in `projects/`);
the desktop app neither opens nor writes those files. It is kept working but
is no longer developed.

It plays two TB-303 voices (a saw bass with distortion and delay, a square
lead with delay and reverb), kick, snare and closed and open hats at 128 BPM,
starting from a short demo pattern.

## Keys

**Everywhere**

| Key | Action |
|---|---|
| Enter | Play / stop |
| Tab | Next view (Sequencer, Synth, Drums, Effects, Mixer, Song) |
| Ctrl+1–6 | Go to a view |
| + / - | Tempo |
| Ctrl+S / Ctrl+L | Save / load project |
| Ctrl+Q | Quit |

**Sequencer**

| Key | Action |
|---|---|
| Arrows | Move around the grid (it wraps at the edges) |
| Space | Toggle step |
| A / S | Accent / slide on the step |
| [ / ] | Previous / next pattern (16 per instrument) |
| { / } | Pattern length: 16, 32 or 64 steps |
| Shift+C | Copy the pattern to the next slot |
| Shift+P | Store the current sound with the pattern |
| N | Note entry (303 tracks): Z–M play notes, S/D/G/H/J sharps, Q/W octave; the cursor moves on after each note |
| G / Shift+G | Dense / sparse acid bassline (303 tracks) |
| Shift+R | Randomise the track's sound |
| Shift+H | Cycle hi-hat patterns (8ths, 16ths, swing, offbeat, sparse) |
| Shift+F | Drum fill (four variations) |
| Shift+B | Build-up (three variations) |
| Shift+K | Four-on-the-floor kick |
| x | Clear the track |
| m / o | Mute / solo the track |

**Synth (TB-303)**: Left/Right picks 303 #1 or #2, Up/Down a control, `<` /
`>` change it, W toggles saw/square, R randomises.

**Drums**: 1–4 pick kick, snare, closed or open hat; Up/Down a control; `<` /
`>` change it; R randomises.

**Effects**: 1–6 pick a channel, Left/Right the effect (distortion, delay,
reverb), Up/Down its controls (on, two settings, mix), `<` / `>` change them, E
switches it on or off.

**Mixer**: 1–6 pick a channel, Up/Down level, pan, mute or solo, `<` / `>`
change them. `m` / `s` mute or solo at the end of the pattern; `M` / `S` at
the end of a four-bar phrase.

## Step symbols

`░` off, `█` on, `▪` accent, `~` slide, `▲` accent and slide. The cursor is
yellow and the playing step red.

## Patterns and projects

Each instrument has 16 patterns, and each pattern can remember its sound, so
switching patterns can change the sound too. A project saves every pattern,
the song, all synth, drum, effect and mixer settings, the tempo and the
selected patterns.

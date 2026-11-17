# Changelog

Entries start at 3.0.0-alpha.59. Earlier changes are in the git history and
the session logs in the `*_PLAN.md` files.

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

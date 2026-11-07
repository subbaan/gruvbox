# Pattern banks and Jam → Song

The user asked (2026-11-07) to lift the 8-pattern limit ("I was already
eyeballing that 8 pattern limit … I guess it's time"), as the groundwork for
recording a Jam into a song. They chose **64 slots as 8 banks (A–H) of 8
pads**.

## Phase 1: 64 pattern slots (alpha.51)

### Model and file format (`groove/`)
- `PatternCount = 64`, `BankSize = 8`, `BankCount = 8`.
- `SlotName(slot)` gives "A1"…"H8", and `BankOf(slot)` gives the bank.
- `FormatVersion` is 2. `Decode` still reads version 1 (8 patterns per track)
  and pads every track to 64 slots.
- `Save` writes a blank pattern (16 default steps, resolution 16, no stored
  state) as `{}`, so a new project stays small. `Decode` turns `{}` back into
  a blank pattern. Steps and resolution are `omitempty` for this.
- A version-2 file can't be opened by alpha.50 or older. They report
  "unsupported project format 2".

### Audio (`groove/audio.go`)
- `Publish` used to prepare a playback bank for every slot. A slot's bank only
  differs from the plain tracks when some track has stored state in it.
- Slots without stored state now share one prepared bank. Only slots with
  state get their own. Publishing costs the same as before for ordinary
  projects.

### Generators
- **Grow** writes the other seven pads of the source's bank. A peak in B3 fills
  B1, B2 and B4–B8. Each bank can hold a grown song.
- **Variation** (action deck) copies to the next pad in the same bank and wraps
  within it.
- **Duplicate or Variation → empty slot** searches forward through all 64
  slots.

### Desktop
- The top bar has a bank button (A–H) before the 8 pads. Its menu marks banks
  that hold patterns.
- Pads show the viewed bank. Choosing a pattern in another bank (from the
  song, history or Grow) moves the view to it.
- Keys 1–8 pick a pad in the viewed bank, and Alt+1–8 view bank A–H.
- Song sections pick "A1"…"H8", and the timeline labels sections the same way.
- The patterns menu copies to slots through one submenu per bank.
- Status lines, labels and export file names use slot names.

### Memory
Undo keeps 64 whole-project snapshots. Each 10-track project with 64 slots of
16 steps is about 0.75 MB, so roughly 50 MB at most. That is acceptable. If it
ever isn't, share unchanged pattern slices between snapshots.

## Phase 2: Jam → Song (next)

- A **Record** toggle sits beside Jam. While it's on, each Jam change is
  captured with the bar it happened at. Stopping shows a card ("12 changes
  over 96 bars") with Apply and undo.
- Captured states are written into the free slots of a bank, or of the next
  empty bank.
- A change that only brought a part in or dropped one out becomes a section
  with mutes on the previous pattern, not a new slot.
- If there are more distinct states than free pads, the most similar
  neighbours are merged first.
- Work out these merging rules before building, since they decide whether
  the result sounds like the jam.

## Session log

- **2026-11-07 (alpha.51):** Phase 1 done as described above.
  - **Groove tests:** `groove/banks_test.go` covers slot names, migrating a v1 file, small saves with an exact round trip, shared banks and Grow staying in its bank.
  - **Desktop:**
    - `bankView`/`bankFollows` and `showBankMenu`/`showBank` are in `desktop/ui.go`, and Alt+1–8 are canvas shortcuts.
    - The patterns menu has one copy submenu per bank, and filled slots are marked "● replace".
    - Tests: `TestPatternPadsAndNumberKeys` covers banks, the view following the pattern and Variation wrapping; the Grow test checks bank A only.
  - **Measured** with 10 tracks and a grown song: Clone 0.28 ms and 0.87 MB, Publish 0.24 ms, a session edit 1.4 ms.
  - **Naming:** slot names ("A1") look like the note names in the 303's step cells. Context keeps them apart ("Pattern A1"); revisit if it confuses.

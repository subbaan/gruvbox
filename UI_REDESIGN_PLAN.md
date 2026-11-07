# Desktop UI Redesign Plan

**Status:** in progress, spread over several sessions.
**Scope:** the Fyne desktop GUI in `desktop/` (the default interface). The legacy TUI in `ui/` is out of scope.
**Started:** 2026-11-06 at 3.0.0-alpha.23.

Read this file first in any session that touches `desktop/`. Tick items off
as they land, add a line to the **Session log** at the bottom, and keep the
**Next up** pointer current.

## Goal

Make the desktop UI streamlined and fast to work in, in line with
`GRUVBOX3_PRODUCT_DESIGN.md` ("silence to groove in seconds", "instrument,
not a miniature DAW"). Every action for a track should be near that track,
the music should get most of the screen, and frequent actions should take one
click or one key.

## Problems found (audit at alpha.23)

1. **Scattered controls.** For one track, the generators are in the bottom
   action deck, the sound is in the right-hand inspector, step details are in
   a separate Step tab, the name is in the Mix tab, and patterns are a
   dropdown at the top.
2. **Chrome eats the screen.** Three button rows on top plus the deck,
   status line and a permanent shortcut list at the bottom take about 40% of
   780px. Each inspector slider stacks its label above the slider, so only
   about 5 fit.
3. **Buried groovebox basics.** Patterns are a `01` dropdown, step pages are
   `‹ Steps / Steps ›`, Mute/Solo are tiny letters with hard-coded hit
   areas, and Save asks for a path every time.
4. **Too many OS windows.** Generators, presets, effect LFOs, section tracks,
   recording, export, confirmations and errors each open a native window.
5. **Rebuild-everything refresh.** `refresh()` → `rebuildInspector()`
   recreates every inspector widget on every edit. That wastes CPU, drops
   focus and can make the scroll jump, and it blocks better interactions
   (see Phase 4).
6. **Help paragraphs inline** in the Tom, Length, Song and Perform panels.
7. **Inconsistencies.** While playing there were two "Stop" buttons. Entries
   applied only on Enter and lost edits on blur. Resolution was labelled
   "steps per whole note". The theme name was in the status bar. No version
   was shown on screen.

## Target layout

```
┌ GRUVBOX 3 v… ▶Play  126 BPM  Swing ▬○ │ PATTERN [1][2][3][4][5][6][7][8] │ Pattern|Song ↶ ↷ ≡ ? ┐
├──────────────────────────────────────────────────────┬──────────────────────────────────────┤
│ Steps │ Roll │ Song │ Mixer │ Perform  page [1][2][3][4]│ 303-1 · TB-303            [Presets] │
│ ▌303-1    M S  █ ░ █ ░ ░ █ █ █ █ ░ █ █ █ █ ░ ░         │ [Acid][Sparse][Busy][Mutate] ⋯       │
│ ▌Kick     M S  █ ░ ░ ░ █ ░ ░ ░ █ ░ ░ ░ █ ░ ░ ░         │ Sound │ Seq │ LFO │ FX               │
│                                                       │ Cutoff     ━━━━━━○━━━  0.55          │
│                                                       │ Envelope   ━━━━━━○━━━  0.60  L       │
├───────────────────────────────────────────────────────┴──────────────────────────────────────┤
│ status / last action                                                                       │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Phases

Each phase stands alone: bump the version, `go build -o gruvbox`, and
`go test ./desktop/ ./groove/` must pass before the phase is called done.

### Phase 1: Quick wins (no architectural change)

- [x] 1.1 Ctrl+S / Save writes straight to the known path; Save as… (Ctrl+Shift+S) asks for one.
- [x] 1.2 Single Play/Stop toggle (removed the duplicate Stop button).
- [x] 1.3 Pattern pads 1–8 replace the dropdown: current pattern highlighted, empty slots dimmed, queued slot marked. "Store state" moved out of the transport (it stays in Patterns ▸ and Ctrl+Shift+P).
- [x] 1.4 Step page buttons replace `‹ Steps / Steps ›`. Pages beyond the longest pattern are hidden, and the page under the selected track's playhead is marked `▸`.
- [x] 1.5 One-line sliders (name | slider | value) in `sliderRow`.
- [x] 1.6 Entries apply on blur as well as Enter (`commitEntry` in `desktop/widgets.go`): BPM, pattern length, track name, section name, section bars.
- [x] 1.7 Inline help paragraphs replaced by `?` popover buttons (`u.helpButton`).
- [x] 1.8 Permanent shortcut strip replaced by a `?` shortcuts overlay in the transport. Theme name dropped from the status bar. Version shown next to the brand.
- [x] 1.9 Keys: 1–8 select a pattern; `[` / `]` change step page.

### Phase 2: Incremental refresh (foundation for phases 3 to 5)

- [x] 2.1 `refresh()` split into `refreshDeck`, `refreshInspector`, `refreshSong` and `refreshTransport`.
- [x] 2.2 Inspector widgets stay alive and are synced in place. They are rebuilt only when `inspectorShape` changes: theme, track ID, kind, sample pointer, LFO sync modes, effect IDs/kinds/order, delay sync. Lock value rows always exist and are shown or hidden, so locking no longer rebuilds. Focus, scroll and the Dual accordion state survive edits.
- [x] 2.3 Song panel (keyed by section IDs) and FX panel (inside the inspector shape) are incremental too. The deck is keyed by track ID, kind and name.
- [x] 2.4 Tests in `desktop/incremental_test.go`. `BenchmarkStepToggle`: in-place 0.77 ms vs rebuild 31 ms per step toggle (about 40×).

### Phase 3: Track-centred layout

- [x] 3.1 One top bar. New ▸ / Open / Save / Save as / Export WAV ▸ / Appearance live in the `≡` menu (`showMainMenu`); Undo/Redo are icon buttons. The project name and play state moved to the status line.
- [x] 3.2 Generators sit in the inspector header (`generatorHeader`, a 4-column grid). Clear, Euclidean, Locks and Phrase are under `⋯`. The bottom deck and its duplicate "Randomise sound" are gone.
- [x] 3.3 Right-clicking a track name opens a menu: rename (popup entry), duplicate, colour (swatch icons), move up/down, mute, solo, remove. The Duplicate/Remove/↑/↓ buttons are gone; `+ Track` and `Patterns` stay above the editor. *Double-click to rename was skipped:* a Fyne widget that implements DoubleTappable delays every single tap, which would make step toggling feel laggy.
- [x] 3.4 Mute/Solo are drawn chips (Error/Focus fill when on) over the hit areas `muteLeft` / `soloLeft` in `grid.go`.
- [x] 3.5 Mixer editor tab (`desktop/mixer.go`): a strip per track with name (click to select), vertical level, pan, and M/S. It is bound, so it only rebuilds when tracks are added, removed or reordered. The Mix inspector tab is gone, so the inspector tabs are now Sound(0) Step(1) Seq(2) LFO(3) FX(4).
- [x] 3.6 Pattern / Song toggle beside the pads (`setSongMode`, `updateModeButtons` on the tick). Song is disabled until sections exist. The "Play song" checkbox is gone.

### Phase 4: Unified sound and step editing

- [x] 4.1 Lock mode (`u.lockMode`): a "Lock step NN" check in the inspector header, or **K**; **Esc** leaves it. Lockable Sound, Dual, sampler gain and FX controls (including the delay division select) write `step.Locks` for the selected step and show that step's values. In lock mode a grid or roll click selects the step instead of editing it. The status line shows `LOCK · step NN`. Shift-drag was not done: Fyne sliders don't expose modifiers.
- [x] 4.2 Names of controls locked on the selected step are bold and use `HighImportance` (primary colour). Non-lockable controls are dimmed (`LowImportance`) in lock mode. Double-clicking a name (`paramName`, DoubleTappable on the label only, so step taps aren't delayed) clears the shown lock, or resets the base value to `Parameter.Default`.
- [x] 4.3 The Step tab keeps the step fields, then LOCKS: only the step's locks, as value sliders with ×, plus **Add lock** (a menu of every lockable parameter, which covers anything not drawn as a slider) and **Clear locks**. The long checkbox list is gone. The grid marks locks on rests with a dim **L** (FX locks act on rests).

### Phase 5: In-window panels and grid polish

- [x] 5.1 Generators (Euclidean, Locks, Phrase), Presets and FX LFOs open in an in-window drawer instead of native windows. Keep native windows for file pickers and recording. *Done alpha.28: `desktop/drawer.go`. The drawer replaces the inspector in the side pane (a Stack of `u.sidePane` and `u.drawerHost`) and narrows the split to `drawerOffset` (.56), restoring it on close. `openDrawer(title, closed)` returns `(*drawer, fresh)` like `u.panel`; `drawer` has `SetContent`/`Content`/`Close`, and `closed` runs on every close (×, Esc, replaced by another drawer, main window closing). Tests use `u.drawerTitled(title)`. Conf, downloads, file pickers, recording, export and section tracks stay native.*
- [x] 5.2 Errors and short confirmations become an in-window banner/toast or overlay rather than a native window. *Done alpha.28: `desktop/modal.go`. `confirm` is a modal card over the main window (`confirmOn(canvas, …)` for utility windows; recording uses it). Cards are tracked in `u.modals[title]` (its `Close()` declines once) and `u.modalOrder`; `u.key` sends keys to `modalKey` first, so Esc declines, Enter accepts, and other shortcuts are swallowed. The card focuses its first Entry and Enter there accepts. `showError` shows a banner above the status line (`errorToast`), fades after 10 s, × dismisses; `u.relayout()` re-runs the root Border layout because showing a hidden child does not resize its parents (the software painter crashes on the zero-sized icon otherwise).*
- [x] 5.3 Grid: highlight the whole playing column, show velocity as fill height or brightness, and tint pages that contain notes. *Done alpha.29: each track's playhead marker is now a lit, outlined cell; `gridRenderer.column` highlights the step number for the selected track's position. `velocityShade` (grid and roll) fades the track colour toward the empty-step colour by up to 60% as velocity drops (`mixHex`/`translucent` in `theme.go`). `u.pageFilled` (filled in `refresh`) dims empty step pages.*
- [x] 5.4 Piano roll: shorter rows (about 24px) so more of the range fits, and a vertical scroll that follows the notes. *Done alpha.30: `rollRows` = 36 (three octaves) at `rollRowHeight` = 22 (`grid.go`); `g.rowH()` / `g.cellBox()` give the per-grid geometry, and playhead `y` now includes the cell inset. Rows are shaded by key (`blackKey`) with a line at each octave; marks sit on the note's line. `fitNotes` centres `noteTop` on the selected track's notes when the track/pattern key (`rollKey`) changes; `followRoll` (called at the end of `refresh` and on editor-tab change) scrolls `u.rollScroll` to them once the roll has a size. A phrase apply clears `rollKey` to refit. Octave shifts persist until the key changes.*
- [x] 5.5 Song tab as a horizontal block timeline (block width = bars, click to jump/select), with the existing row editor for the selected section. *Done alpha.31: `desktop/timeline.go` (`songTimeline`, retained `lit`/`marker` moved by `advance()` from the 60 Hz tick in `updatePlaybackLabel`). The rows all stay (tests index `songPanel.Objects[1+i]`); the row number became a button, lit for `u.selectedSection`, and `selectSection` scrolls `u.songScroll` to the row. `refreshSong` keeps `selectedSection` valid (falls back to the first section). Jumping playback to a section is not supported by the player yet, so a click only selects.*
- [x] 5.6 FX chain as collapsible cards (header: enable, name, ↑↓, LFO, ×). *Done alpha.32 in `fx.go`: card = Stack(panel rectangle, Padded(VBox(header, body))); fold state is view-only in `u.fxFolded[effectID]` and survives rebuilds; the title dims when the unit or chain is off; the LFO button lights when either LFO is enabled. Top rows: Bypass + Randomise, then kind select + Add. Found and fixed a bug class while here: `u.bound` only registers a sync, so styling done in a bound closure was wrong until the next refresh (mixer M/S and name highlight, Clear locks enabled with no locks). New `u.live(f)` runs the sync and binds it; use it for any styling sync written inline.*
- [x] 5.7 More keys: Tab cycles editor tabs, `,`/`.` nudge velocity, Ctrl+C/V copy/paste steps. *Done alpha.33 in `desktop/clipboard.go`. Plain Tab is taken by Fyne's focus traversal (glfw `capturesTab`), so editor tabs use Ctrl+Tab / Ctrl+Shift+Tab. `,`/`.` nudge velocity by 0.1 (0.1–1). Ctrl+C/V copy/paste the selected step (paste advances); Ctrl+Shift+C/V copy/paste the track's pattern (steps + step size, not the pattern's sound state); `keepLocks` drops locks the target cannot use. **Bug fixed:** the desktop driver sends Ctrl+Z/Y/C/V as `fyne.ShortcutUndo/Redo/Copy/Paste`, not `desktop.CustomShortcut`, so plain Ctrl+Z undo never fired in the real app (tests called `u.undo()` directly). These names are now bound; tests in `keys_test.go` dispatch them through the canvas the way the driver does.*

## Conventions for this work

- Patch-bump the version for every change (`VERSION`, `config/config.go`, `README.md`, `CLAUDE.md`) and check they agree.
- The audio-thread rules in `CLAUDE.md` still apply. UI code talks to audio only through `u.player` / `Publish`.
- All edits go through `u.edit(label, fn)` so undo works. Sliders use `bindLiveSlider` for one undo per gesture.
- Tests: `go test ./desktop/ ./groove/`. Tests drive widgets directly (e.g. `u.selectPattern(1)`), so update them when widgets change.
- **Seeing the UI (preferred):** `GRUVBOX_SHOTS=<scratch dir> go test ./desktop -run TestScreenshots` renders the window with Fyne's software painter (`desktop/redesign_test.go`) and writes PNGs. It is headless and can be driven freely, so extend it with new views as phases land.
- **Seeing the real app:** `./gruvbox -no-audio &`, find the window with `xdotool search --name '^Gruvbox 3'`, capture with `import -window <id> file.png` into the session scratchpad, then kill the process. Don't drive the user's mouse with xdotool. There is no Xvfb on this machine.
- `desktop/helpers_test.go` has `findSlider(root, namePrefix)`. Use it instead of matching container shapes in tests.
- Fyne 2.7 has no built-in tooltips. Use `u.helpButton(text)` popovers.
- **Binding rule (since Phase 2):** controls in the inspector or song panel must use the `bound*` helpers in `desktop/bind.go` (`boundSlider`, `boundSliderFormat`, `boundCheck`, `boundSelect`, `boundEntry`, `boundLabel`, `boundVisible`, or `u.bound(func)` for custom widgets). Their getters must read the live project (`u.session.Project`, `u.selectedStep`), never values captured at build time. Callbacks must capture the pattern slot and step index when the user acts. If a condition changes *which* widgets exist, add it to `inspectorShape` (or show and hide with `boundVisible`). A plain `sliderRow` inside the inspector would go stale after undo.
- Utility windows (generators, FX LFOs, presets) still rebuild themselves. `u.bind` is nil outside a panel build, so bound helpers used there register nothing.

## Where things live (after Phase 5)

- `desktop/ui.go`: window layout (`content`), `refresh`, keys (`key`), save/save as (`save`, `saveAs`, `writeProject`), pattern pads (`selectPattern`, `updatePatternPads`), page buttons (`setPage`, `updatePageButtons`), `shortcutHelp` text. Keep `shortcutHelp` in sync when keys change.
- `desktop/widgets.go`: `commitEntry` (apply on Enter or blur), `helpButton` / `showHelp` / `withHelp` (popover help).
- `desktop/bind.go`: `binder`, `u.building`, `u.bound`, the `set*` no-callback setters, and the `bound*` widget helpers.
- `desktop/inspector.go`: `sliderParts` / `sliderRow` (one-line slider), `inspectorShape`, `refreshInspector` (sync or rebuild), `rebuildInspector` (Sound/Step/Seq/Mix tabs). `u.inspectorBuilds` counts rebuilds for tests.
- `desktop/fx.go`, `modulation.go` (`lfoControls(get, …)`), `dual.go`, `sample.go`: bound panels used by the inspector. `sample.go` also syncs the waveform's trim markers.
- `desktop/song.go`: `refreshSong` / `rebuildSong`.
- `desktop/locks.go`: lock mode (`setLockMode`, `currentStep`, `editStep`, `setLock`, `clearLock`, `locked`), `paramControl` + `paramRow` (lock-aware slider, optional log mapping via `toSlider`/`fromSlider`), `paramSelect`, `paramName` (double-click label), `lockToggle`, `stepLocks`, `shortLockName`. New Sound/FX parameters should use `paramRow` (or `u.instrumentParam` in `inspector.go`), not `boundSlider`, so they take part in locking.
- `desktop/mixer.go`: `refreshMixer` / `rebuildMixer` / `mixerStrip`, `selectTrack`.
- `desktop/trackmenu.go`: `showTrackMenu`, `renameTrack`, `swatch` (SVG colour icon for menus).
- `desktop/templates.go` / `export.go`: `templateItems()` / `exportItems()` feed the `≡` menu.
- `desktop/song.go`: `updatePlaybackLabel` runs on the 60 Hz playhead tick and also refreshes pads and pages. Anything added there must check for changes before touching widgets.
- `desktop/drawer.go`: `openDrawer`, `closeDrawer`, `drawerTitled`. Use a drawer for any new tool panel; use `u.panel` only for things that need their own native window.
- `desktop/timeline.go`: `songTimeline`, `selectSection`.
- `desktop/clipboard.go`: step/pattern copy-paste, `nudgeVelocity`, `cycleEditorTab`.
- `desktop/bind.go`: `u.live(f)` = run now + bind; prefer it over `u.bound` for inline styling syncs.
- `desktop/modal.go`: `confirm` / `confirmOn`, `modalKey`, `showError` (toast), `hideToast`, `relayout`.
- `groove.PatternEmpty(p, slot)` was added for the pads; `NextEmptyPattern` now uses it.

## Next up

All five phases are done (alpha.24 to alpha.33). Nothing is queued; ask the
user what they want next. Candidate follow-ups noticed along the way:

- Check drawers, confirm cards and the error banner by hand in the real app
  (so far they are verified by tests and headless screenshots only).
- Song timeline: a click could also jump playback to that section once the
  player can start at a given section (`groove` transport work).
- Piano roll: the step-number header scrolls away with the rows; it could be
  pinned above the scroll like the Song timeline.
- Shift-drag lock (4.1) and double-click rename (3.3) remain skipped for the
  Fyne reasons noted there.
- Inspector width: the FX cards' right edge sits under the scrollbar at the
  default split; consider padding the side pane's scroll content.

## Session log

- **2026-11-06 (alpha.24):** audit written; Phase 1 done (1.1 to 1.9). Also: inspector title is one line, "Resolution" became "Step size" (1/4 to 1/64), section bars reject values below 1, BPM entry rejects values outside 40 to 300, and the help popover has an opaque panel (the theme's overlay colour is translucent on purpose for menus). Tests added in `desktop/redesign_test.go`. Added the `-V` version flag (global rule).
- **2026-11-06 (alpha.25):** Phase 2 done (2.1 to 2.4). Every inspector, FX, LFO, Dual, sampler and song control is bound. Side fixes: the sampler Start/End sliders read the live opposite marker (they used a stale captured track), and the FX "Add" kind selector keeps its choice across edits. The condition select gained a "Play" label; the LFO selects gained Target/Wave/Sync/Division labels.
- **2026-11-06 (alpha.26):** Phase 3 done (3.1 to 3.6; double-click rename skipped, see 3.3). Tests in `desktop/layout_test.go`. `TestScreenshots` now also writes `menu`, `trackmenu` and `mixer`. The status line no longer repeats the play state and has no trailing separator. Swing label and slider now follow undo; the BPM field is no longer overwritten while you type in it.
- **2026-11-06 (alpha.27):** Phase 4 done (4.1 to 4.3). Tests in `desktop/locks_test.go`. `TestScreenshots` also writes `lock` and `steplocks`. `findSlider` recognises `paramName` labels. Sampler gain/fade undo labels changed from "Sample gain_db" style to "Set Gain (dB)".
- **2026-11-06 (alpha.28):** 5.1 and 5.2 done. Drawers in `desktop/drawer.go` (tests: `drawer_test.go`); confirm cards and error toast in `desktop/modal.go` (tests: `modal_test.go`). Generator/phrase/FX-LFO tests now find their controls via `u.drawerTitled`; confirm tests use `u.modals`. `TestScreenshots` also writes `drawer-euclid`, `drawer-presets`, `drawer-phrase` and `confirm`.
- **2026-11-06 (alpha.29):** 5.3 done. Lit playing cell plus step-number marker, velocity shading, dimmed empty pages. Tests in `grid_polish_test.go`; `TestPlayheadFollowsAudioWithoutRebuildingGrid` checks the new marker position and the column highlight. `TestScreenshots` also writes `playing`.
- **2026-11-06 (alpha.30):** 5.4 done. Compact three-octave roll, keyboard shading, fit-and-follow. Tests: `TestRollFitsAndFollowsNotes` (in `grid_polish_test.go`); `notes_test.go` now uses `rollRowHeight` and pins `g.noteTop = 59` after resizing. `TestScreenshots` also writes `roll`.
- **2026-11-06 (alpha.31):** 5.5 done. Song timeline (`timeline.go`, tests `timeline_test.go`); screenshot `song`.
- **2026-11-06 (alpha.32):** 5.6 done. FX cards; `u.live` and the initial-state fix for mixer/locks widgets (tests `fxcards_test.go`, including `TestBoundWidgetsStartInState`). `TestEffectsDiscoveryHistoryAndControls` taps the bypass check at its vertical centre (`test.Tap` clicks at (1,1), outside a check's active band when the row is taller). Screenshot `fx`.
- **2026-11-06 (alpha.33):** 5.7 done, Phase 5 complete. Clipboard and keys (`clipboard.go`, tests `keys_test.go`); Ctrl+Z/Ctrl+Y fixed in the real app; shortcut help updated.
- **2026-11-07 (alpha.48):** Readable highlights in every theme (user: a focused select was "white on yellow … almost every theme"). Fyne draws ordinary Foreground text on `ColorNameFocus` (a focused Select, the active popup-menu item) and on Hover/Selection. Focus was the solid accent, which gave 1.0–2.3:1 contrast. In `desktop/theme.go`, `focusTint` mixes the accent into the panel or background at up to 50%, as strongly as keeps the text at ≥ 4.5:1. `readableFill` does the same for Hover/Pressed/Selection when they fall short. Primary stays the accent. `theme.Scheme.TextOn` falls back to black or white when no palette neutral reaches 4.5:1 (primary buttons in Gruvbox Light and Nebula were 3.9). `theme.Contrast` is exported. Test: `TestThemeTextStaysReadableOnHighlights` (`desktop/contrast_test.go`) covers the bundled themes, the light testdata and a dim synthetic theme. Measured on the user's 13 themes: every highlight is now 4.5–5.0:1 or better.
- **2026-11-07 (alpha.49):** A Plica-style Open/Save dialog replaces Fyne's file picker and the Save-As text box everywhere. Plica is Rust, so this is a Go port of its design, not its code. It has three parts:
  - The model: `desktop/filebrowser.go`, with no Fyne in it. It covers places, history, crumbs, sorting with folders first, filtering by type and by the name field, typed paths, bookmarks (`<data root>/bookmarks.json`) and recent files (`recent-files.json`, kept per list and never stat'ed).
  - The previews: `desktop/filepreview.go` describes projects, WAVs, sound presets and themes.
  - The Fyne side: `desktop/filedialog.go`. `u.chooseFile(fileSpec, done)` shows a modal on any canvas. It has `modal.key`/`modal.file` hooks in `modal.go`, `nameEntry` catches the dialog's keys, and `crumbLayout` folds leading crumbs into "…". The software painter crashes on a nil rectangle fill, so the dialog uses `color.Transparent`. Folder reads and previews run off the UI thread. `fileReadAsync = false` in TestMain, because the test driver runs `fyne.Do` inline.

  Callers:
  - `open`/`openProject` and `saveAs`/`writeProject` (which remember recent files);
  - `loadSample`;
  - sound preset import and export;
  - `saveRecordedWAV`;
  - theme import;
  - export's Browse….

  `fileOpen` is gone. Tests: `filebrowser_test.go` (model) and `filedialog_test.go`: Save as with the replace confirm, open by a typed path and from Recent, keys, double click, bookmarks in the data root and the narrow crumb bar. `TestFileDialogScreenshots` is behind `GRUVBOX_SHOTS`. Possible follow-ups: drag-and-drop of files onto the window, and a "Recent" list on the main menu.
- **2026-11-07 (alpha.51):** Pattern banks: a bank button (A–H) before the 8 pads, Alt+1–8, slot names (A1…H8) in song sections, the timeline, menus and status lines. See `PATTERN_BANKS_PLAN.md`.
- **2026-11-07 (alpha.52):** Utility windows (Export WAV, Record sample, Conf, Download themes, Section tracks) sometimes ended up behind the main window. The user runs xfwm4 with focus following the mouse (`click_to_focus=false`), `raise_on_focus=true` and `prevent_focus_stealing=true`, and GLFW windows carry no `_NET_WM_USER_TIME`. So when the pointer passed over the main window, xfwm4 auto-raised it over the new window.
  - **Fix:** `u.showPanel(w)` in `windows.go` calls `raiseOver` (`raise_x11.go`, using pure-Go xgb so X errors are values, not exits). It sets `WM_TRANSIENT_FOR` to the main window, then, once the window is mapped, sends `_NET_ACTIVE_WINDOW` with source 2 (user request).
  - **Other platforms:** `raise_other.go` falls back to RequestFocus.
  - **Verified** on a private Xvfb :77 with xfwm4 using a copy of the user's xfwm4.xml: alpha.51's Export WAV was buried after a pointer move; the fixed build keeps it on top.

#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
phrase_data="$(mktemp -d /tmp/gruvbox-phrase-smoke.XXXXXX)"
mkdir -p "$project_root/artifacts"
export LANG=en_GB.UTF-8 LC_ALL=en_GB.UTF-8
export XDG_CONFIG_HOME="$phrase_data/config" XDG_CACHE_HOME="$phrase_data/cache"
"$project_root/gruvbox" --no-audio --theme-dir "$phrase_data/themes" > "$project_root/artifacts/phrase-smoke.log" 2>&1 &
gui_pid=$!
cleanup() {
    kill "$gui_pid" 2>/dev/null || true
    wait "$gui_pid" 2>/dev/null || true
}
trap cleanup EXIT
window_id=""
for attempt in {1..100}; do
    window_id="$(xdotool search --name 'Gruvbox 3' 2>/dev/null | head -1 || true)"
    if [[ -n "$window_id" ]]; then break; fi
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/phrase-smoke.log"; exit 1; fi
    sleep .2
done
[[ -n "$window_id" ]]
sleep 2
save_project() {
    local destination="$1" save_id
    xdotool windowfocus "$window_id"
    xdotool mousemove --window "$window_id" 1123 68 click 1
    sleep .2
    save_id="$(xdotool search --name '^Save groove$' | head -1)"
    xdotool windowfocus "$save_id"
    xdotool mousemove --window "$save_id" 160 62 click 1
    xdotool key --clearmodifiers ctrl+a
    xdotool type --clearmodifiers -- "$destination"
    xdotool mousemove --window "$save_id" 94 106 click 1
    for attempt in {1..100}; do
        if [[ -s "$destination" ]]; then break; fi
        sleep .1
    done
    [[ -s "$destination" ]]
}
save_project "$phrase_data/before.gruvbox3"
# Keep playback running while applying drafts.
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 192 28 click 1
xdotool mousemove --window "$window_id" 330 112 click 1
sleep .2
xdotool mousemove --window "$window_id" 410 223 click 1
sleep .3
phrase_id="$(xdotool search --name '^Phrase generator$' | head -1)"
xdotool windowmove "$phrase_id" 160 100
xdotool getwindowgeometry --shell "$phrase_id" > "$project_root/artifacts/phrase-geometry.txt"
rg -q '^X=160$' "$project_root/artifacts/phrase-geometry.txt"
rg -q '^Y=100$' "$project_root/artifacts/phrase-geometry.txt"
# Select four bars; the preview grows to four rows and the body scrolls.
xdotool mousemove --window "$phrase_id" 170 214 click 1
xdotool mousemove --window "$phrase_id" 100 312 click 1
sleep .2
import -window "$phrase_id" "$project_root/artifacts/desktop-phrase-four-bars.png"
xdotool mousemove --window "$phrase_id" 410 674 click 1
save_project "$phrase_data/draft.gruvbox3"
cmp "$phrase_data/before.gruvbox3" "$phrase_data/draft.gruvbox3"
xdotool windowfocus "$phrase_id"
xdotool mousemove --window "$phrase_id" 170 712 click 1
save_project "$phrase_data/four.gruvbox3"
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 1233 68 click 1
save_project "$phrase_data/undo.gruvbox3"
cmp "$phrase_data/before.gruvbox3" "$phrase_data/undo.gruvbox3"
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 1289 68 click 1
save_project "$phrase_data/redo.gruvbox3"
cmp "$phrase_data/four.gruvbox3" "$phrase_data/redo.gruvbox3"
# Feel only retains the applied notes. Pitches only retains their articulation.
xdotool windowfocus "$phrase_id"
xdotool mousemove --window "$phrase_id" 170 294 click 1
xdotool mousemove --window "$phrase_id" 100 392 click 1
xdotool mousemove --window "$phrase_id" 570 674 click 1
xdotool mousemove --window "$phrase_id" 170 712 click 1
save_project "$phrase_data/feel.gruvbox3"
xdotool windowfocus "$phrase_id"
xdotool mousemove --window "$phrase_id" 170 294 click 1
xdotool mousemove --window "$phrase_id" 100 356 click 1
xdotool mousemove --window "$phrase_id" 410 674 click 1
xdotool mousemove --window "$phrase_id" 170 712 click 1
save_project "$phrase_data/pitches.gruvbox3"
# Restore whole phrase, then resize to two bars and one bar.
xdotool windowfocus "$phrase_id"
xdotool mousemove --window "$phrase_id" 170 294 click 1
xdotool mousemove --window "$phrase_id" 100 320 click 1
xdotool mousemove --window "$phrase_id" 170 358 click 1
xdotool mousemove --window "$phrase_id" 100 420 click 1
sleep .2
import -window "$phrase_id" "$project_root/artifacts/desktop-phrase-two-bars.png"
xdotool mousemove --window "$phrase_id" 170 712 click 1
save_project "$phrase_data/two.gruvbox3"
xdotool windowfocus "$phrase_id"
xdotool mousemove --window "$phrase_id" 170 262 click 1
xdotool mousemove --window "$phrase_id" 100 290 click 1
xdotool mousemove --window "$phrase_id" 170 712 click 1
save_project "$phrase_data/one.gruvbox3"
python3 - "$phrase_data" <<'PHRASECHECK'
import json, os, sys
root = sys.argv[1]
def load(name):
    with open(os.path.join(root, name + '.gruvbox3')) as f:
        return json.load(f)
before, four, feel, pitches, two, one = [load(name) for name in ('before', 'four', 'feel', 'pitches', 'two', 'one')]
for snapshot, length in ((four, 64), (feel, 64), (pitches, 64), (two, 32), (one, 16)):
    assert snapshot['tracks'][1:] == before['tracks'][1:]
    track = snapshot['tracks'][0]
    assert track['parameters'] == before['tracks'][0]['parameters']
    assert track['patterns'][1:] == before['tracks'][0]['patterns'][1:]
    pattern = track['patterns'][0]
    assert pattern['resolution'] == 16 and len(pattern['steps']) == length, length
    if snapshot in (four, two, one):
        for bar in range(length // 16):
            assert sum(step['active'] for step in pattern['steps'][bar*16:(bar+1)*16]) == 8
    assert all(step['note'] in (36, 39, 41, 43, 46, 48) for step in pattern['steps'])
old, changed, pitched = [s['tracks'][0]['patterns'][0]['steps'] for s in (four, feel, pitches)]
for a, b, c in zip(old, changed, pitched):
    assert (a['active'], a['note']) == (b['active'], b['note'])
    assert {k:v for k,v in b.items() if k != 'note'} == {k:v for k,v in c.items() if k != 'note'}
assert old != changed
PHRASECHECK
kill -0 "$gui_pid"
printf 'Native phrase window, 1/2/4 bars, partial modes, drafts, Apply and undo/redo succeeded.\n'

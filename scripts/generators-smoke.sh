#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
generator_data="$(mktemp -d /tmp/gruvbox-generators-smoke.XXXXXX)"
mkdir -p "$project_root/artifacts"
export LANG=en_GB.UTF-8 LC_ALL=en_GB.UTF-8
export XDG_CONFIG_HOME="$generator_data/config" XDG_CACHE_HOME="$generator_data/cache"
"$project_root/gruvbox" --no-audio --theme-dir "$generator_data/themes" > "$project_root/artifacts/generator-smoke.log" 2>&1 &
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
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/generator-smoke.log"; exit 1; fi
    sleep .2
done
[[ -n "$window_id" ]]
sleep 2
save_project() {
    local destination="$1"
    xdotool windowfocus "$window_id"
    xdotool mousemove --window "$window_id" 1123 68 click 1
    sleep .2
    local save_id
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
save_project "$generator_data/before.gruvbox3"
xdotool mousemove --window "$window_id" 192 28 click 1
xdotool mousemove --window "$window_id" 330 112 click 1
sleep .2
xdotool mousemove --window "$window_id" 400 149 click 1
sleep .3
euclid_id="$(xdotool search --name '^Euclidean rhythm$' | head -1)"
xdotool windowmove "$euclid_id" 160 100
xdotool getwindowgeometry --shell "$euclid_id" > "$project_root/artifacts/euclidean-geometry.txt"
rg -q '^X=160$' "$project_root/artifacts/euclidean-geometry.txt"
rg -q '^Y=100$' "$project_root/artifacts/euclidean-geometry.txt"
xdotool mousemove --window "$euclid_id" 275 227 click 1
xdotool mousemove --window "$euclid_id" 139 301 click 1
import -window "$euclid_id" "$project_root/artifacts/desktop-euclidean.png"
save_project "$generator_data/draft.gruvbox3"
cmp "$generator_data/before.gruvbox3" "$generator_data/draft.gruvbox3"
xdotool mousemove --window "$euclid_id" 160 622 click 1
save_project "$generator_data/rhythm.gruvbox3"
xdotool windowfocus "$euclid_id"
xdotool key Escape
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 330 112 click 1
sleep .2
xdotool mousemove --window "$window_id" 410 186 click 1
sleep .3
lock_id="$(xdotool search --name '^Parameter-lock generator$' | head -1)"
xdotool windowmove "$lock_id" 160 100
import -window "$lock_id" "$project_root/artifacts/desktop-lock-generator.png"
xdotool mousemove --window "$lock_id" 325 712 click 1
save_project "$generator_data/locks.gruvbox3"
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 1233 68 click 1
save_project "$generator_data/undo.gruvbox3"
cmp "$generator_data/rhythm.gruvbox3" "$generator_data/undo.gruvbox3"
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 1289 68 click 1
save_project "$generator_data/redo.gruvbox3"
cmp "$generator_data/locks.gruvbox3" "$generator_data/redo.gruvbox3"
# Random previews and Reroll must not edit the project until Apply.
xdotool mousemove --window "$lock_id" 300 126 click 1
sleep .2
xdotool mousemove --window "$lock_id" 120 160 click 1
xdotool mousemove --window "$lock_id" 100 712 click 1
import -window "$lock_id" "$project_root/artifacts/desktop-lock-random.png"
save_project "$generator_data/random-draft.gruvbox3"
cmp "$generator_data/locks.gruvbox3" "$generator_data/random-draft.gruvbox3"
xdotool mousemove --window "$lock_id" 325 712 click 1
save_project "$generator_data/random.gruvbox3"
python3 - "$generator_data" <<'GENERATORCHECK'
import json, math, os, sys
root = sys.argv[1]
def load(name):
    with open(os.path.join(root, name + '.gruvbox3')) as f:
        return json.load(f)
before, rhythm, locks, random = [load(name) for name in ('before', 'rhythm', 'locks', 'random')]
assert before['tracks'][1:] == rhythm['tracks'][1:]
steps = rhythm['tracks'][0]['patterns'][0]['steps']
assert len(steps) == 16
assert sum(step['active'] for step in steps) == 7
for old, new in zip(before['tracks'][0]['patterns'][0]['steps'], steps):
    for key in ('note', 'length', 'accent', 'slide', 'probability', 'every'):
        assert old.get(key) == new.get(key)
for snapshot in (rhythm, locks, random):
    assert snapshot['tracks'][0]['parameters'] == before['tracks'][0]['parameters']
values = []
random_values = []
for i, (step, locked, randomized) in enumerate(zip(steps, locks['tracks'][0]['patterns'][0]['steps'], random['tracks'][0]['patterns'][0]['steps'])):
    if step['active']:
        value = locked['locks']['cutoff']
        assert .3 <= value <= .8
        assert abs(value - (.3 + .5 * i / 15)) <= .00501
        values.append(value)
        random_values.append(randomized['locks']['cutoff'])
    else:
        assert 'cutoff' not in locked.get('locks', {})
assert values != random_values
assert all(.3 <= value <= .8 for value in random_values)
GENERATORCHECK
kill -0 "$gui_pid"
printf 'Native Euclidean/lock previews, moveable window, exact Apply, Reroll, save and undo/redo succeeded.\n'

#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
state_data="$(mktemp -d /tmp/gruvbox-pattern-state.XXXXXX)"
mkdir -p "$project_root/artifacts"
export LANG=en_GB.UTF-8 LC_ALL=en_GB.UTF-8
export XDG_CONFIG_HOME="$state_data/config" XDG_CACHE_HOME="$state_data/cache"
"$project_root/gruvbox" --no-audio --theme-dir "$state_data/themes" > "$project_root/artifacts/pattern-state-smoke.log" 2>&1 &
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
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/pattern-state-smoke.log"; exit 1; fi
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
select_pattern() {
    xdotool windowfocus "$window_id"
    xdotool mousemove --window "$window_id" 650 28 click 1
    xdotool mousemove --window "$window_id" 645 "$1" click 1
    sleep .1
}
save_project "$state_data/before.gruvbox3"
xdotool windowfocus "$window_id"
xdotool key --clearmodifiers ctrl+shift+p
save_project "$state_data/first.gruvbox3"
xdotool mousemove --window "$window_id" 1233 68 click 1
save_project "$state_data/undo.gruvbox3"
cmp "$state_data/before.gruvbox3" "$state_data/undo.gruvbox3"
xdotool mousemove --window "$window_id" 1289 68 click 1
save_project "$state_data/redo.gruvbox3"
cmp "$state_data/first.gruvbox3" "$state_data/redo.gruvbox3"
select_pattern 91
# Mute the first track and change its cutoff, then capture using the button.
xdotool mousemove --window "$window_id" 178 228 click 1
xdotool mousemove --window "$window_id" 1230 293 click 1
xdotool mousemove --window "$window_id" 785 28 click 1
save_project "$state_data/second.gruvbox3"
select_pattern 55
save_project "$state_data/return.gruvbox3"
import -window "$window_id" "$project_root/artifacts/desktop-pattern-state-stored.png"
# A live mute is visible as edited; leaving and returning restores the snapshot.
xdotool mousemove --window "$window_id" 178 228 click 1
save_project "$state_data/live.gruvbox3"
import -window "$window_id" "$project_root/artifacts/desktop-pattern-state-edited.png"
select_pattern 91
select_pattern 55
save_project "$state_data/unstored-return.gruvbox3"
cmp "$state_data/return.gruvbox3" "$state_data/unstored-return.gruvbox3"
xdotool mousemove --window "$window_id" 178 228 click 1
xdotool mousemove --window "$window_id" 785 28 click 1
save_project "$state_data/updated.gruvbox3"
xdotool mousemove --window "$window_id" 1233 68 click 1
save_project "$state_data/undo-update.gruvbox3"
cmp "$state_data/live.gruvbox3" "$state_data/undo-update.gruvbox3"
xdotool mousemove --window "$window_id" 1289 68 click 1
save_project "$state_data/redo-update.gruvbox3"
cmp "$state_data/updated.gruvbox3" "$state_data/redo-update.gruvbox3"
python3 - "$state_data" <<'STATECHECK'
import json, os, sys
root = sys.argv[1]
def load(name):
    with open(os.path.join(root, name + '.gruvbox3')) as f:
        return json.load(f)
before, first, second, returned, live, updated = [load(n) for n in ('before', 'first', 'second', 'return', 'live', 'updated')]
assert all('state' not in t['patterns'][0] for t in before['tracks'])
assert all('state' in t['patterns'][0] for t in first['tracks'])
assert second['pattern'] == 1 and returned['pattern'] == 0
original = first['tracks'][0]['patterns'][0]['state']
changed = second['tracks'][0]['patterns'][1]['state']
assert not original['mute'] and changed['mute']
assert changed['parameters']['cutoff'] != original['parameters']['cutoff']
for key in ('mute', 'solo', 'level', 'pan', 'parameters', 'lfos', 'effects_bypassed'):
    assert returned['tracks'][0].get(key) == original.get(key)
assert live['tracks'][0]['mute'] and not live['tracks'][0]['patterns'][0]['state']['mute']
assert updated['tracks'][0]['patterns'][0]['state']['mute']
for old, saved in zip(before['tracks'], first['tracks']):
    for a,b in zip(old['patterns'], saved['patterns']):
        assert a['steps'] == b['steps'] and a['resolution'] == b['resolution']
STATECHECK
kill -0 "$gui_pid"
printf 'Native pattern state shortcut/button, independent mutes and sounds, recall, explicit overwrite and undo/redo succeeded.\n'

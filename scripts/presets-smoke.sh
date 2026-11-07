#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
preset_data="$(mktemp -d /tmp/gruvbox-presets-smoke.XXXXXX)"
mkdir -p "$project_root/artifacts"
export XDG_CONFIG_HOME="$preset_data/config"
export XDG_CACHE_HOME="$preset_data/cache"
export LANG=en_GB.UTF-8
export LC_ALL=en_GB.UTF-8
"$project_root/gruvbox" --no-audio --theme-dir "$preset_data/themes" > "$project_root/artifacts/preset-smoke.log" 2>&1 &
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
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/preset-smoke.log"; exit 1; fi
    sleep 0.2
done
[[ -n "$window_id" ]]
sleep 2
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 192 28 click 1
xdotool key r
xdotool mousemove --window "$window_id" 1248 181 click 1
xdotool mousemove --window "$window_id" 1130 219 click 1
xdotool mousemove --window "$window_id" 1018 181 click 1
xdotool mousemove --window "$window_id" 1240 219 click 1
sleep 0.5
preset_id="$(xdotool search --name '^Sound library$' | head -1)"
[[ "$preset_id" != "$window_id" ]]
xdotool windowmove "$preset_id" 180 140
xdotool getwindowgeometry --shell "$preset_id" > "$project_root/artifacts/preset-geometry.txt"
rg -q '^X=180$' "$project_root/artifacts/preset-geometry.txt"
rg -q '^Y=140$' "$project_root/artifacts/preset-geometry.txt"
import -window "$preset_id" "$project_root/artifacts/desktop-sound-library.png"
xdotool mousemove --window "$preset_id" 290 499 click 1
sleep 0.3
save_id="$(xdotool search --name '^Save sound preset$' | head -1)"
xdotool windowfocus "$save_id"
xdotool mousemove --window "$save_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- 'Found sweep'
import -window "$save_id" "$project_root/artifacts/desktop-save-sound.png"
xdotool mousemove --window "$save_id" 94 106 click 1
for attempt in {1..100}; do
    if compgen -G "$preset_data/sounds/*.gruvsound" > /dev/null; then break; fi
    sleep 0.1
done
python3 - "$preset_data/sounds" <<'PRESETCHECK'
import glob, json, os, sys
files = glob.glob(os.path.join(sys.argv[1], '*.gruvsound'))
assert len(files) == 1
with open(files[0]) as f:
    preset = json.load(f)
assert preset['sound_preset_version'] == 1
assert preset['name'] == 'Found sweep'
assert preset['kind'] == 'tb303'
assert len(preset['effects']) == 4
assert len(preset['lfos']) == 2
PRESETCHECK
sleep 0.3
import -window "$preset_id" "$project_root/artifacts/desktop-saved-preset.png"
# Recall a built-in while playing, export it, then return to the saved discovery.
xdotool mousemove --window "$preset_id" 120 170 click 1
sleep 0.3
xdotool mousemove --window "$preset_id" 100 499 click 1
xdotool mousemove --window "$preset_id" 100 535 click 1
sleep 0.3
export_id="$(xdotool search --name '^Export sound preset$' | head -1)"
xdotool windowfocus "$export_id"
xdotool mousemove --window "$export_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$preset_data/rounded.gruvsound"
xdotool mousemove --window "$export_id" 100 106 click 1
for attempt in {1..100}; do
    if [[ -s "$preset_data/rounded.gruvsound" ]]; then break; fi
    sleep 0.1
done
python3 - "$preset_data/rounded.gruvsound" <<'EXPORTCHECK'
import json, sys
with open(sys.argv[1]) as f:
    preset = json.load(f)
assert preset['name'] == 'Rounded sub'
assert preset['parameters']['cutoff'] == .3
assert not preset.get('effects')
EXPORTCHECK
xdotool windowfocus "$window_id"
xdotool key --clearmodifiers ctrl+z
xdotool windowfocus "$preset_id"
xdotool key Escape
xdotool mousemove --window "$window_id" 1240 219 click 1
sleep 0.5
preset_id="$(xdotool search --name '^Sound library$' | head -1)"
xdotool windowfocus "$preset_id"
xdotool mousemove --window "$preset_id" 120 62 click 1
xdotool type --clearmodifiers -- 'Found sweep'
sleep 0.3
xdotool mousemove --window "$preset_id" 120 98 click 1
sleep 0.3
xdotool mousemove --window "$preset_id" 100 499 click 1
import -window "$preset_id" "$project_root/artifacts/desktop-preset-selected.png"
import -window "$window_id" "$project_root/artifacts/desktop-preset-recalled.png"
xdotool mousemove --window "$preset_id" 100 535 click 1
sleep 0.3
export_id="$(xdotool search --name '^Export sound preset$' | head -1)"
xdotool windowfocus "$export_id"
xdotool mousemove --window "$export_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$preset_data/found.gruvsound"
xdotool mousemove --window "$export_id" 100 106 click 1
for attempt in {1..100}; do
    if [[ -s "$preset_data/found.gruvsound" ]]; then break; fi
    sleep 0.1
done
python3 - "$preset_data" <<'RECALLCHECK'
import glob, json, os, sys
with open(glob.glob(os.path.join(sys.argv[1], 'sounds', '*.gruvsound'))[0]) as f:
    saved = json.load(f)
with open(os.path.join(sys.argv[1], 'found.gruvsound')) as f:
    exported = json.load(f)
assert saved == exported
RECALLCHECK
kill -0 "$gui_pid"
printf 'Native preset save, search, live recall, undo, reopen and portable export succeeded.\n'

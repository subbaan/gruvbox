#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
dual_data="$(mktemp -d /tmp/gruvbox-dual-smoke.XXXXXX)"
mkdir -p "$project_root/artifacts"
export LANG=en_GB.UTF-8 LC_ALL=en_GB.UTF-8
export XDG_CONFIG_HOME="$dual_data/config" XDG_CACHE_HOME="$dual_data/cache"
"$project_root/gruvbox" --no-audio --theme-dir "$dual_data/themes" > "$project_root/artifacts/dual-smoke.log" 2>&1 &
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
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/dual-smoke.log"; exit 1; fi
    sleep 0.2
done
[[ -n "$window_id" ]]
sleep 2
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 42 112 click 1
sleep 0.2
xdotool mousemove --window "$window_id" 35 466 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 318 674 click 1
xdotool mousemove --window "$window_id" 208 492 click 1
xdotool mousemove --window "$window_id" 192 28 click 1
import -window "$window_id" "$project_root/artifacts/desktop-dual-sound.png"
xdotool mousemove --window "$window_id" 1070 420 mousedown 1
for sweep_x in 1100 1140 1180 1220 1270; do
    xdotool mousemove --window "$window_id" "$sweep_x" 420
    sleep 0.04
done
xdotool mouseup 1
xdotool key --window "$window_id" ctrl+z
xdotool mousemove --window "$window_id" 1240 219 click 1
sleep 0.5
preset_id="$(xdotool search --name '^Sound library$' | head -1)"
import -window "$preset_id" "$project_root/artifacts/desktop-dual-presets.png"
xdotool mousemove --window "$preset_id" 120 98 click 1
sleep 0.3
xdotool mousemove --window "$preset_id" 100 499 click 1
xdotool windowfocus "$preset_id"
xdotool key Escape
xdotool mousemove --window "$window_id" 1206 181 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 1030 219 click 1
import -window "$window_id" "$project_root/artifacts/desktop-dual-lfo.png"
xdotool mousemove --window "$window_id" 110 148 click 1
import -window "$window_id" "$project_root/artifacts/desktop-dual-piano.png"
# Validate a saved project from native controls, not just a screenshot.
xdotool mousemove --window "$window_id" 1123 68 click 1
sleep 0.3
save_id="$(xdotool search --name '^Save groove$' | head -1)"
xdotool windowfocus "$save_id"
xdotool mousemove --window "$save_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$dual_data/dual.gruvbox3"
xdotool mousemove --window "$save_id" 94 106 click 1
for attempt in {1..100}; do
    if [[ -s "$dual_data/dual.gruvbox3" ]]; then break; fi
    sleep 0.1
done
python3 - "$dual_data/dual.gruvbox3" <<'PROJECTCHECK'
import json, sys
with open(sys.argv[1]) as f:
    project = json.load(f)
track = project['tracks'][-1]
assert track['kind'] == 'dual-synth'
assert track['parameters']['osc2_level'] == .6
assert track['lfos'][0]['enabled']
assert track['solo']
assert any(step['active'] for step in track['patterns'][0]['steps'])
PROJECTCHECK
xdotool mousemove --window "$window_id" 1179 68 click 1
sleep 0.2
xdotool mousemove --window "$window_id" 1220 106 click 1
sleep 0.3
export_id="$(xdotool search --name '^Export WAV$' | head -1)"
xdotool windowfocus "$export_id"
xdotool mousemove --window "$export_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$dual_data/dual.wav"
xdotool mousemove --window "$export_id" 38 298 click 1
for attempt in {1..100}; do
    if [[ -s "$dual_data/dual.wav" ]]; then break; fi
    sleep 0.1
done
python3 - "$dual_data/dual.wav" <<'WAVCHECK'
import array, sys, wave
with wave.open(sys.argv[1], 'rb') as wav:
    assert wav.getnchannels() == 2
    assert wav.getframerate() == 44100
    audio = array.array('h', wav.readframes(wav.getnframes()))
    assert max(abs(sample) for sample in audio) > 100
WAVCHECK
cp "$dual_data/dual.wav" "$project_root/artifacts/dual-preview.wav"
printf 'Native Dual track, live cutoff, preset recall, LFO, piano roll, save and solo WAV export succeeded.\n'

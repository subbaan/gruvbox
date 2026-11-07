#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
smoke_data="$(mktemp -d /tmp/gruvbox-gui-smoke.XXXXXX)"
mkdir -p "$project_root/artifacts"
export XDG_CONFIG_HOME="$smoke_data/config"
export XDG_CACHE_HOME="$smoke_data/cache"
export LANG=en_GB.UTF-8
export LC_ALL=en_GB.UTF-8
"$project_root/gruvbox" --no-audio --theme-dir "$smoke_data/themes" > "$project_root/artifacts/gui-smoke.log" 2>&1 &
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
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/gui-smoke.log"; exit 1; fi
    sleep 0.2
done
if [[ -z "$window_id" ]]; then cat "$project_root/artifacts/gui-smoke.log"; exit 1; fi
sleep 2
xdotool mousemove --window "$window_id" 42 112 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-add-track.png"
if xdotool search --name '^Add track$' >/dev/null 2>&1; then exit 1; fi
xdotool key --window "$window_id" Escape
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-warm.png"
xdotool mousemove --window "$window_id" 1024 68 click 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-new-templates.png"
xdotool key --window "$window_id" Escape
xdotool mousemove --window "$window_id" 1290 28 click 1
sleep 1
conf_id="$(xdotool search --name '^Conf / Appearance$' | head -1)"
if [[ "$conf_id" == "$window_id" ]]; then exit 1; fi
xdotool windowmove "$conf_id" 180 140
xdotool getwindowgeometry --shell "$conf_id" > "$project_root/artifacts/conf-geometry.txt"
rg -q '^X=180$' "$project_root/artifacts/conf-geometry.txt"
rg -q '^Y=140$' "$project_root/artifacts/conf-geometry.txt"
import -window "$conf_id" "$project_root/artifacts/desktop-conf.png"
xdotool windowfocus "$conf_id"
xdotool key Escape
xdotool key --window "$window_id" r
xdotool key --window "$window_id" g
xdotool key --window "$window_id" ctrl+z
sleep 1
xdotool mousemove --window "$window_id" 1070 367 mousedown 1
for sweep_x in 1080 1120 1160 1200 1240 1270; do
    xdotool mousemove --window "$window_id" "$sweep_x" 367
    sleep 0.04
done
import -window "$window_id" "$project_root/artifacts/desktop-live-sweep.png"
xdotool mouseup 1
xdotool key --window "$window_id" ctrl+z
xdotool mousemove --window "$window_id" 1206 181 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-lfo.png"
xdotool mousemove --window "$window_id" 1248 181 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 1130 219 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-fx.png"
xdotool mousemove --window "$window_id" 110 148 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 210 266 click 1
xdotool mousemove --window "$window_id" 210 266 mousedown 1
for drag_xy in '235 280' '270 310' '300 340' '326 354'; do
    read -r drag_x drag_y <<< "$drag_xy"
    xdotool mousemove --window "$window_id" "$drag_x" "$drag_y"
    sleep 0.06
done
xdotool mouseup 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-piano.png"
xdotool mousemove --window "$window_id" 1233 68 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-piano-undo.png"
xdotool mousemove --window "$window_id" 166 148 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 195 190 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-song.png"
xdotool mousemove --window "$window_id" 667 282 click 1
sleep 0.5
section_id="$(xdotool search --name '^Section tracks' | head -1)"
import -window "$section_id" "$project_root/artifacts/desktop-section-tracks.png"
xdotool mousemove --window "$section_id" 301 392 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 1179 68 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 1220 145 click 1
sleep 0.5
export_id="$(xdotool search --name '^Export WAV$' | head -1)"
import -window "$export_id" "$project_root/artifacts/desktop-export.png"
xdotool mousemove --window "$export_id" 280 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$smoke_data/smoke-song.wav"
xdotool mousemove --window "$export_id" 38 298 click 1
for attempt in {1..100}; do
    if [[ -s "$smoke_data/smoke-song.wav" ]]; then break; fi
    sleep 0.1
done
[[ -s "$smoke_data/smoke-song.wav" ]]
python3 - "$smoke_data/smoke-song.wav" <<'WAVCHECK'
import sys, wave
with wave.open(sys.argv[1], 'rb') as audio:
    assert audio.getnchannels() == 2
    assert audio.getsampwidth() == 2
    assert audio.getframerate() == 44100
    assert audio.getnframes() > 44100
WAVCHECK
cp "$smoke_data/smoke-song.wav" "$project_root/artifacts/smoke-song.wav"
sleep 0.3
import -window "$export_id" "$project_root/artifacts/desktop-export-complete.png"
xdotool mousemove --window "$export_id" 110 298 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 42 112 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 35 394 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 1018 181 click 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-tom-sound.png"
xdotool mousemove --window "$window_id" 110 148 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 210 266 click 1
sleep 0.2
xdotool mousemove --window "$window_id" 210 266 mousedown 1
xdotool mousemove --window "$window_id" 390 332
sleep 0.2
xdotool mouseup 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-tom-piano-roll.png"
xdotool mousemove --window "$window_id" 42 112 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 35 502 click 1
sleep 0.5
xdotool mousemove --window "$window_id" 210 266 click 1
sleep 0.2
xdotool mousemove --window "$window_id" 240 266 mousedown 1
xdotool mousemove --window "$window_id" 357 266
sleep 0.2
xdotool mouseup 1
sleep 0.3
xdotool mousemove --window "$window_id" 500 310 click 1
sleep 0.2
xdotool windowfocus "$window_id"
xdotool keydown Shift
sleep 0.2
xdotool mousemove --window "$window_id" 210 266 click 1
sleep 0.2
xdotool mousemove --window "$window_id" 500 310 click 1
sleep 0.2
xdotool keyup Shift
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-note-length-selection.png"
xdotool mousemove --window "$window_id" 230 148 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 192 28 click 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-perform-ready.png"
xdotool mousemove --window "$window_id" 60 290 click 1
sleep 0.5
import -window "$window_id" "$project_root/artifacts/desktop-perform-queued.png"
xdotool mousemove --window "$window_id" 238 28 click 1
xdotool mousemove --window "$window_id" 1248 181 click 1
sleep 0.3
xdotool mousemove --window "$window_id" 1110 303 click 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-effect-types.png"
xdotool mousemove --window "$window_id" 1100 467 click 1
xdotool mousemove --window "$window_id" 1289 294 click 1
sleep 0.3
import -window "$window_id" "$project_root/artifacts/desktop-chorus-controls.png"
xdotool mousemove --window "$window_id" 1140 407 click 1
sleep 0.5
fx_mod_id="$(xdotool search --name 'modulation$' | head -1)"
xdotool windowmove "$fx_mod_id" 180 140
import -window "$fx_mod_id" "$project_root/artifacts/desktop-fx-modulation.png"
kill -0 "$gui_pid"
printf 'Native GUI editing, movable settings, song section controls and WAV export succeeded. Capture: %s\n' "$project_root/artifacts/desktop-warm.png"

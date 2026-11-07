#!/usr/bin/env bash
# Synthetic capture checks the native workflow without recording real inputs.
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
record_data="$(mktemp -d /tmp/gruvbox-recording-smoke.XXXXXX)"
mkdir -p "$record_data/tools" "$project_root/artifacts"
export XDG_CONFIG_HOME="$record_data/config" XDG_CACHE_HOME="$record_data/cache"
export LANG=en_GB.UTF-8 LC_ALL=en_GB.UTF-8
export GRUVBOX_CAPTURE_TEST_PID="$record_data/helper.pid"
cat > "$record_data/tools/pactl" <<'SOURCES'
#!/bin/sh
printf '%s\n' '[{"name":"test.monitor","description":"Synthetic output","monitor_source":"test"},{"name":"test-input","description":"Synthetic input"}]'
SOURCES
cat > "$record_data/tools/parec" <<'SIGNAL'
#!/usr/bin/env python3
import math, os, struct, sys, time
channels = int(next(arg.split('=')[1] for arg in sys.argv if arg.startswith('--channels=')))
with open(os.environ['GRUVBOX_CAPTURE_TEST_PID'], 'w') as f:
    f.write(str(os.getpid()))
phase = 0
while True:
    block = bytearray()
    for i in range(882):
        value = int(14000 * math.sin(2 * math.pi * (phase + i) * 440 / 44100))
        for ch in range(channels):
            block.extend(struct.pack('<h', value if ch == 0 else -value))
    os.write(1, block)
    phase += 882
    time.sleep(.02)
SIGNAL
chmod +x "$record_data/tools/pactl" "$record_data/tools/parec"
export PATH="$record_data/tools:$PATH"
"$project_root/gruvbox" --no-audio --theme-dir "$record_data/themes" > "$project_root/artifacts/recording-smoke.log" 2>&1 &
gui_pid=$!
cleanup() {
    kill "$gui_pid" 2>/dev/null || true
    wait "$gui_pid" 2>/dev/null || true
    if [[ -f "$record_data/helper.pid" ]]; then kill "$(cat "$record_data/helper.pid")" 2>/dev/null || true; fi
}
trap cleanup EXIT
window_id=""
for attempt in {1..100}; do
    window_id="$(xdotool search --name 'Gruvbox 3' 2>/dev/null | head -1 || true)"
    if [[ -n "$window_id" ]]; then break; fi
    if ! kill -0 "$gui_pid" 2>/dev/null; then cat "$project_root/artifacts/recording-smoke.log"; exit 1; fi
    sleep .2
done
[[ -n "$window_id" ]]
sleep 2
xdotool windowfocus "$window_id"
xdotool mousemove --window "$window_id" 42 112 click 1
sleep .2
xdotool mousemove --window "$window_id" 35 502 click 1
sleep .3
xdotool mousemove --window "$window_id" 1240 257 click 1
sleep .5
record_id="$(xdotool search --name '^Record sample$' | head -1)"
[[ "$record_id" != "$window_id" ]]
xdotool windowmove "$record_id" 180 90
xdotool getwindowgeometry --shell "$record_id" > "$project_root/artifacts/recording-geometry.txt"
rg -q '^X=180$' "$project_root/artifacts/recording-geometry.txt"
rg -q '^Y=90$' "$project_root/artifacts/recording-geometry.txt"
import -window "$record_id" "$project_root/artifacts/desktop-record-ready.png"
xdotool mousemove --window "$record_id" 150 173 click 1
sleep .8
import -window "$record_id" "$project_root/artifacts/desktop-record-active.png"
xdotool mousemove --window "$record_id" 450 173 click 1
sleep .6
import -window "$record_id" "$project_root/artifacts/desktop-record-take.png"
# Save the original WAV, then trim both ends in the recording preview.
xdotool mousemove --window "$record_id" 150 632 click 1
sleep .3
save_id="$(xdotool search --name '^Save recording WAV$' | head -1)"
xdotool windowfocus "$save_id"
xdotool mousemove --window "$save_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$record_data/stereo.wav"
xdotool mousemove --window "$save_id" 94 106 click 1
for attempt in {1..100}; do
    if [[ -s "$record_data/stereo.wav" ]]; then break; fi
    sleep .1
done
xdotool mousemove --window "$record_id" 34 366 mousedown 1
xdotool mousemove --window "$record_id" 120 366
sleep .2
xdotool mouseup 1
xdotool mousemove --window "$record_id" 566 366 mousedown 1
xdotool mousemove --window "$record_id" 500 366
sleep .2
xdotool mouseup 1
xdotool mousemove --window "$record_id" 90 594 click 1
xdotool mousemove --window "$record_id" 500 594 click 1
sleep .3
import -window "$record_id" "$project_root/artifacts/desktop-record-trim.png"
import -window "$window_id" "$project_root/artifacts/desktop-record-loaded.png"
xdotool windowfocus "$window_id"
xdotool key --clearmodifiers ctrl+z
xdotool key --clearmodifiers ctrl+shift+z
xdotool mousemove --window "$window_id" 1123 68 click 1
sleep .3
save_id="$(xdotool search --name '^Save groove$' | head -1)"
xdotool windowfocus "$save_id"
xdotool mousemove --window "$save_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$record_data/take.gruvbox3"
xdotool mousemove --window "$save_id" 94 106 click 1
for attempt in {1..100}; do
    if [[ -s "$record_data/take.gruvbox3" ]]; then break; fi
    sleep .1
done
python3 - "$record_data" <<'TAKECHECK'
import base64, json, os, sys, wave
root = sys.argv[1]
with wave.open(os.path.join(root, 'stereo.wav'), 'rb') as wav:
    assert wav.getnchannels() == 2 and wav.getframerate() == 44100
    assert wav.getnframes() > 4410
with open(os.path.join(root, 'take.gruvbox3')) as f:
    track = json.load(f)['tracks'][-1]
assert track['kind'] == 'sampler'
assert .05 < track['parameters']['start'] < .4
assert .6 < track['parameters']['end'] < .99
with open(os.path.join(root, 'stereo.wav'), 'rb') as f:
    assert f.read() == base64.b64decode(track['sample']['wav'])
TAKECHECK
# A mono take stops automatically after five captured seconds.
xdotool mousemove --window "$record_id" 150 101 click 1
sleep .2
xdotool mousemove --window "$record_id" 150 174 click 1
xdotool mousemove --window "$record_id" 470 101 click 1
sleep .2
xdotool mousemove --window "$record_id" 470 138 click 1
xdotool mousemove --window "$record_id" 150 173 click 1
sleep 5.8
import -window "$record_id" "$project_root/artifacts/desktop-record-limit.png"
xdotool mousemove --window "$record_id" 150 632 click 1
sleep .3
save_id="$(xdotool search --name '^Save recording WAV$' | head -1)"
xdotool windowfocus "$save_id"
xdotool mousemove --window "$save_id" 160 62 click 1
xdotool key --clearmodifiers ctrl+a
xdotool type --clearmodifiers -- "$record_data/mono.wav"
xdotool mousemove --window "$save_id" 94 106 click 1
for attempt in {1..100}; do
    if [[ -s "$record_data/mono.wav" ]]; then break; fi
    sleep .1
done
python3 - "$record_data/mono.wav" <<'LIMITCHECK'
import sys, wave
with wave.open(sys.argv[1], 'rb') as wav:
    assert wav.getnchannels() == 1
    assert wav.getnframes() == 44100 * 5
LIMITCHECK
# Native close cancels an active capture and reaps its helper.
xdotool mousemove --window "$record_id" 150 173 click 1
sleep .3
helper_pid="$(cat "$record_data/helper.pid")"
xdotool windowfocus "$record_id"
xdotool key Escape
for attempt in {1..30}; do
    if ! kill -0 "$helper_pid" 2>/dev/null; then break; fi
    sleep .1
done
if kill -0 "$helper_pid" 2>/dev/null; then exit 1; fi
kill -0 "$gui_pid"
printf 'Native stereo/mono recording, meter, trim, audition control, WAV save, use/undo/redo, automatic limit and close cancellation succeeded.\n'

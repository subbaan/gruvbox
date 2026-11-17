package groove

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// TailMode says what an export does with effect tails (reverb, delay).
type TailMode int

const (
	// TailAppend renders tailSeconds more after the music, with the notes
	// released, so the effects ring out at the end of the file.
	TailAppend TailMode = iota
	// TailWrap makes a seamless loop: the pattern first plays silently for at
	// least tailSeconds (whole cycles, see loopCycle), so the effects carry
	// the previous pass's tail into the start, then exactly one loop is
	// recorded. Pattern exports only.
	TailWrap
)

// maxCycleLoops bounds the warm-up cycle for a wrapped loop; patterns whose
// tracks only line up again after more loops warm up in whole loops.
const maxCycleLoops = 16

// ExportWAV uses a separate offline engine and atomically replaces the target
// only after rendering, writing and syncing successfully. Playback stays live.
func ExportWAV(ctx context.Context, path string, project Project, song bool, tailSeconds float64, progress func(float64)) error {
	return ExportWAVMode(ctx, path, project, song, tailSeconds, TailAppend, progress)
}

// ExportWAVMode is ExportWAV with a choice of what to do with the tail.
func ExportWAVMode(ctx context.Context, path string, project Project, song bool, tailSeconds float64, mode TailMode, progress func(float64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := exportLength(project, song, tailSeconds, mode); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gruvbox-wav-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = RenderWAVMode(ctx, file, project, song, tailSeconds, mode, progress)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// exportSize is how an export is laid out in frames: warm silent frames
// (wrapped loops only), then frames of music and tail frames of effect tail.
type exportSize struct{ warm, frames, tail int64 }

func exportLength(project Project, song bool, tailSeconds float64, mode TailMode) (exportSize, error) {
	frames, tail, exact, err := exportFrames(project, song, tailSeconds)
	if err != nil || mode != TailWrap {
		return exportSize{frames: frames, tail: tail}, err
	}
	if song {
		return exportSize{}, fmt.Errorf("only a pattern can be exported as a loop")
	}
	size := exportSize{frames: frames}
	if tail > 0 {
		cycle := float64(loopCycle(project))
		passes := math.Ceil(float64(tail)/(cycle*exact)) * cycle
		size.warm = int64(math.Round(passes * exact))
	}
	return size, nil
}

// loopCycle is how many loops of an exported pattern it takes for every
// track to be back on its first step, with every "every N" step due, at the
// same time: 1 when the tracks share one length and no step skips loops. It
// is 1 when that takes more than maxCycleLoops loops, or when an odd-length
// pattern is swung (its length then falls between ticks).
func loopCycle(project Project) int {
	var longest, cycle int64 = 0, 1
	for _, track := range project.Tracks {
		pattern := track.Patterns[project.Pattern]
		if len(pattern.Steps)%2 != 0 && project.Swing != 0 {
			return 1
		}
		// In 1/64 notes; resolutions are 4 to 64.
		ticks := int64(len(pattern.Steps) * 64 / pattern.Resolution)
		every := int64(1)
		for _, step := range pattern.Steps {
			if step.Active && step.Every > 1 {
				every = lcm(every, int64(step.Every))
			}
		}
		longest = max(longest, ticks)
		if cycle = lcm(cycle, ticks*every); cycle > 1<<40 {
			return 1
		}
	}
	if longest == 0 || cycle/longest > maxCycleLoops {
		return 1
	}
	return int(cycle / longest)
}

func lcm(a, b int64) int64 {
	x, y := a, b
	for y != 0 {
		x, y = y, x%y
	}
	return a / x * b
}

// exportFrames is the music's length in frames (rounded up, and exact) and the
// tail's.
func exportFrames(project Project, song bool, tailSeconds float64) (int64, int64, float64, error) {
	if err := project.Validate(); err != nil {
		return 0, 0, 0, err
	}
	if !inRange(tailSeconds, 0, 10) {
		return 0, 0, 0, fmt.Errorf("effect tail must be 0–10 seconds")
	}
	beats := 0.
	if song {
		if len(project.Song) == 0 {
			return 0, 0, 0, fmt.Errorf("add song sections before exporting a song")
		}
		for _, section := range project.Song {
			beats += float64(section.Repeats * 4)
		}
	} else {
		for _, track := range project.Tracks {
			pattern := track.Patterns[project.Pattern]
			length := float64(len(pattern.Steps)) * 4 / float64(pattern.Resolution)
			if len(pattern.Steps)%2 != 0 {
				length += project.Swing * 4 / float64(pattern.Resolution)
			}
			beats = max(beats, length)
		}
		if beats == 0 {
			return 0, 0, 0, fmt.Errorf("add tracks before exporting a loop")
		}
	}
	exact := beats * SampleRate * 60 / project.BPM
	frames := int64(math.Ceil(exact))
	tail := int64(math.Round(tailSeconds * SampleRate))
	if (frames+tail)*4 > math.MaxUint32-36 {
		return 0, 0, 0, fmt.Errorf("export exceeds the WAV 4 GiB limit; shorten the song")
	}
	return frames, tail, exact, nil
}

// RenderWAV writes signed 16-bit stereo at 44.1 kHz with bounded memory.
func RenderWAV(ctx context.Context, writer io.Writer, project Project, song bool, tailSeconds float64, progress func(float64)) error {
	return RenderWAVMode(ctx, writer, project, song, tailSeconds, TailAppend, progress)
}

// RenderWAVMode is RenderWAV with a choice of what to do with the tail.
func RenderWAVMode(ctx context.Context, writer io.Writer, project Project, song bool, tailSeconds float64, mode TailMode, progress func(float64)) error {
	size, err := exportLength(project, song, tailSeconds, mode)
	if err != nil {
		return err
	}
	frames, tail := size.frames, size.tail
	if err = ctx.Err(); err != nil {
		return err
	}
	var header [44]byte
	copy(header[:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+(frames+tail)*4))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], SampleRate)
	binary.LittleEndian.PutUint32(header[28:], SampleRate*4)
	binary.LittleEndian.PutUint16(header[32:], 4)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32((frames+tail)*4))
	if err = writeWAVBytes(writer, header[:]); err != nil {
		return err
	}
	player := NewPlayer()
	player.Publish(project)
	player.SetSongMode(song)
	player.SetPlaying(true)
	var audio [BlockSize * 2]float32
	var encoded [BlockSize * 4]byte
	total, work := frames+tail, size.warm+frames+tail
	done := int64(0)
	lastProgress := -1.
	// The master bus lags by masterLatency frames: drop its first output and
	// flush it at the end, so the file lines up with the groove (an exported
	// loop still starts on its downbeat and loops cleanly). A wrapped loop
	// also drops its silent warm-up passes.
	skip, written := int64(masterLatency)+size.warm, int64(0)
	emit := func(out []float32) error {
		player.master.process(out)
		drop := min(skip, int64(len(out)/2))
		skip -= drop
		keep := min(int64(len(out)/2)-drop, total-written)
		for i, sample := range out[drop*2 : (drop+keep)*2] {
			binary.LittleEndian.PutUint16(encoded[i*2:], uint16(int16(sample*32767)))
		}
		written += keep
		return writeWAVBytes(writer, encoded[:keep*4])
	}
	for _, phase := range []struct {
		count int64
		tail  bool
	}{{size.warm + frames, false}, {tail, true}} {
		if phase.tail && phase.count > 0 {
			player.endExportNotes()
		}
		remaining := phase.count
		for remaining > 0 {
			if err = ctx.Err(); err != nil {
				return err
			}
			count := int(min(remaining, BlockSize))
			out := audio[:count*2]
			if phase.tail {
				player.renderTail(out)
			} else {
				player.renderMix(out)
			}
			if err = emit(out); err != nil {
				return err
			}
			remaining -= int64(count)
			done += int64(count)
			fraction := float64(done) / float64(work)
			if progress != nil && (fraction-lastProgress >= .01 || done == work) {
				progress(fraction)
				lastProgress = fraction
			}
		}
	}
	// The master's look-ahead sees what follows the file: silence after a
	// tail, the loop's own start again for a wrapped loop.
	flush := audio[:masterLatency*2]
	if mode == TailWrap {
		player.renderMix(flush)
	} else {
		clear(flush)
	}
	return emit(flush)
}
func writeWAVBytes(writer io.Writer, data []byte) error {
	n, err := writer.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
func (p *Player) endExportNotes() {
	if p.active == nil {
		return
	}
	for i := range p.active.tracks {
		v := p.active.tracks[i].voice
		if v.acid != nil {
			v.acid.NoteOff()
		}
		if v.mono != nil {
			v.mono.NoteOff()
		}
		v.samplePlaying = false
	}
}
func (p *Player) renderTail(output []float32) {
	clear(output)
	if p.active == nil {
		return
	}
	for frame := 0; frame < len(output)/2; frame++ {
		for i := range p.active.tracks {
			left, right := p.active.tracks[i].renderSample()
			output[frame*2] += left
			output[frame*2+1] += right
		}
	}
}
func (t *playTrack) renderSample() (float32, float32) {
	v := t.voice
	v.buffer[0] = 0
	if v.acid != nil {
		v.acid.Process(v.buffer[:])
		v.buffer[0] *= v.velocity
	} else if v.mono != nil {
		v.mono.Process(v.buffer[:])
	} else if v.drum != nil {
		v.drum.Process(v.buffer[:])
	}
	left, right := v.buffer[0], v.buffer[0]
	if v.kind == Sampler {
		left, right = v.sampleVoice.next()
	}
	for _, fx := range t.effects {
		left, right = fx.state.process(left, right)
	}
	return left * t.left, right * t.right
}

// IsolateTrack makes the track with id the only one heard (soloed and
// unmuted, every other solo cleared), for rendering a single track. It reports
// false if no track has that id.
func IsolateTrack(p *Project, id string) bool {
	found := false
	for i := range p.Tracks {
		t := &p.Tracks[i]
		t.Solo = t.ID == id
		if t.Solo {
			t.Mute, found = false, true
		}
	}
	return found
}

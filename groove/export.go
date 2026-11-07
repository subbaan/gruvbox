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

// ExportWAV uses a separate offline engine and atomically replaces the target
// only after rendering, writing and syncing successfully. Playback stays live.
func ExportWAV(ctx context.Context, path string, project Project, song bool, tailSeconds float64, progress func(float64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, _, err := exportFrames(project, song, tailSeconds); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gruvbox-wav-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = RenderWAV(ctx, file, project, song, tailSeconds, progress)
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
func exportFrames(project Project, song bool, tailSeconds float64) (int64, int64, error) {
	if err := project.Validate(); err != nil {
		return 0, 0, err
	}
	if !inRange(tailSeconds, 0, 10) {
		return 0, 0, fmt.Errorf("effect tail must be 0–10 seconds")
	}
	beats := 0.
	if song {
		if len(project.Song) == 0 {
			return 0, 0, fmt.Errorf("add song sections before exporting a song")
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
			return 0, 0, fmt.Errorf("add tracks before exporting a loop")
		}
	}
	frames := int64(math.Ceil(beats * SampleRate * 60 / project.BPM))
	tail := int64(math.Round(tailSeconds * SampleRate))
	if (frames+tail)*4 > math.MaxUint32-36 {
		return 0, 0, fmt.Errorf("export exceeds the WAV 4 GiB limit; shorten the song")
	}
	return frames, tail, nil
}

// RenderWAV writes signed 16-bit stereo at 44.1 kHz with bounded memory.
func RenderWAV(ctx context.Context, writer io.Writer, project Project, song bool, tailSeconds float64, progress func(float64)) error {
	frames, tail, err := exportFrames(project, song, tailSeconds)
	if err != nil {
		return err
	}
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
	total := frames + tail
	done := int64(0)
	lastProgress := -1.
	for _, phase := range []struct {
		count int64
		tail  bool
	}{{frames, false}, {tail, true}} {
		if phase.tail {
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
				player.Render(out)
			}
			for i, sample := range out {
				binary.LittleEndian.PutUint16(encoded[i*2:], uint16(int16(sample*32767)))
			}
			if err = writeWAVBytes(writer, encoded[:count*4]); err != nil {
				return err
			}
			remaining -= int64(count)
			done += int64(count)
			fraction := float64(done) / float64(total)
			if progress != nil && (fraction-lastProgress >= .01 || done == total) {
				progress(fraction)
				lastProgress = fraction
			}
		}
	}
	return nil
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
		if v.dual != nil {
			v.dual.NoteOff()
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
		output[frame*2] = max(-.98, min(.98, output[frame*2]))
		output[frame*2+1] = max(-.98, min(.98, output[frame*2+1]))
	}
}
func (t *playTrack) renderSample() (float32, float32) {
	v := t.voice
	v.buffer[0] = 0
	if v.acid != nil {
		v.acid.Process(v.buffer[:])
		v.buffer[0] *= v.velocity
	} else if v.dual != nil {
		v.dual.Process(v.buffer[:])
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

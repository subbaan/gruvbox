package groove

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSectionMutesRespectSoloAndSurviveHistory(t *testing.T) {
	p := NewProject()
	p.Tracks = p.Tracks[:2]
	p.BPM = 300
	first, second := NewSongSection(0), NewSongSection(0)
	first.Repeats, second.Repeats = 1, 1
	id := p.Tracks[0].ID
	p.Tracks[0].Mute = true
	first.Mutes = map[string]bool{id: false}
	second.Mutes = map[string]bool{id: true}
	p.Song = []SongSection{first, second}
	session := NewSession(p)
	session.Do("Mute", func(p *Project) error { p.Song[0].Mutes[id] = true; return nil })
	if p.Song[0].Mutes[id] {
		t.Fatal("section map aliases history")
	}
	session.Undo()
	if session.Project.Song[0].Mutes[id] {
		t.Fatal("section mute undo failed")
	}
	data, _ := json.Marshal(p)
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil || loaded.Song[0].Mutes[id] {
		t.Fatalf("saved false override lost: %v", err)
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetSongMode(true)
	player.SetPlaying(true)
	renderFrames(player, 1)
	if player.active.tracks[0].left == 0 {
		t.Fatal("section could not unmute globally muted track")
	}
	renderFrames(player, SampleRate*240/300)
	if player.active.tracks[0].left != 0 {
		t.Fatal("next section did not mute track")
	}
	player.SetSongMode(false)
	renderFrames(player, SampleRate*240/300)
	if player.active.tracks[0].left != 0 {
		t.Fatal("song exit lost global mute")
	}
	p.Tracks[1].Solo = true
	player.Publish(p)
	player.SetPlaying(false)
	renderFrames(player, 1)
	player.SetSongMode(true)
	player.SetPlaying(true)
	renderFrames(player, 1)
	if player.active.tracks[0].left != 0 {
		t.Fatal("section mute override bypassed solo")
	}
	p.Song[0].Mutes["missing-track"] = true
	if p.Validate() == nil {
		t.Fatal("dangling section mute accepted")
	}
}
func TestWholeGrooveCopyKeepsIndependentLocksAndPolymeters(t *testing.T) {
	p := NewProject()
	p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{"cutoff": .65}
	p.Tracks[1].Patterns[0].Steps = p.Tracks[1].Patterns[0].Steps[:7]
	p.Tracks[1].Patterns[0].Resolution = 8
	p.Tracks[0].Patterns[0].Steps[0].Active = true
	if NextEmptyPattern(p, 0) != 1 {
		t.Fatal("empty slot discovery failed")
	}
	session := NewSession(p)
	if err := session.Do("Copy", func(p *Project) error { return CopyPattern(p, 0, 1, "") }); err != nil {
		t.Fatal(err)
	}
	t0, t1 := session.Project.Tracks[0], session.Project.Tracks[1]
	if len(t1.Patterns[1].Steps) != 7 || t1.Patterns[1].Resolution != 8 || !t0.Patterns[1].Steps[0].Active {
		t.Fatal("copy lost polymetric content")
	}
	t0.Patterns[1].Steps[0].Locks["cutoff"] = .2
	if t0.Patterns[0].Steps[0].Locks["cutoff"] != .65 {
		t.Fatal("copied pattern locks alias source")
	}
	session.Undo()
	if session.Project.Tracks[0].Patterns[1].Steps[0].Active {
		t.Fatal("copy undo failed")
	}
	for slot := 1; slot < PatternCount; slot++ {
		p.Tracks[0].Patterns[slot].Steps[0].Active = true
	}
	if NextEmptyPattern(p, 0) != -1 {
		t.Fatal("duplicate would overwrite occupied slot")
	}
	if CopyPattern(&p, 0, 0, "") == nil || CopyPattern(&p, 0, 1, "missing") == nil {
		t.Fatal("invalid copy accepted")
	}
}

package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestTemplatesSaveRenderAndRemainIndependent(t *testing.T) {
	counts := map[Template]int{ClassicTemplate: 6, DrumsTemplate: 8, TomBassTemplate: 5, SamplerTemplate: 4, EmptyTemplate: 0}
	for _, template := range Templates {
		t.Run(string(template), func(t *testing.T) {
			p, err := NewFromTemplate(template)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tracks) != counts[template] {
				t.Fatal("unexpected track count")
			}
			encoded, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := Decode(bytes.NewReader(encoded))
			if err != nil || !reflect.DeepEqual(p, loaded) {
				t.Fatalf("template save/load: %v", err)
			}
			second, err := NewFromTemplate(template)
			if err != nil {
				t.Fatal(err)
			}
			for i, track := range p.Tracks {
				if track.ID == second.Tracks[i].ID {
					t.Fatal("templates reuse track identities")
				}
				second.Tracks[i].Parameters[Parameters(track.Kind)[0].ID] = -999
				if p.Tracks[i].Parameters[Parameters(track.Kind)[0].ID] == -999 {
					t.Fatal("templates share sound maps")
				}
			}
			if template == TomBassTemplate && (p.Tracks[0].Kind != Tom || p.Tracks[0].Parameters["pitch_decay"] != 0 || math.Abs(p.Tracks[0].Parameters["pitch"]-65.40639) > .001) {
				t.Fatal("tom template lacks tuned bass tone")
			}
			if template == SamplerTemplate {
				for _, track := range p.Tracks {
					if track.Kind != Sampler || track.Sample != nil {
						t.Fatal("sampler kit did not start with empty slots")
					}
				}
			}
			player := NewPlayer()
			player.Publish(p)
			player.SetPlaying(true)
			out := make([]float32, BlockSize*2)
			energy := 0.
			for block := 0; block < 32; block++ {
				player.Render(out)
				for _, v := range out {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatal("invalid template audio")
					}
					energy += math.Abs(float64(v))
				}
			}
			if template != SamplerTemplate && template != EmptyTemplate && energy == 0 {
				t.Fatal("musical template is silent")
			}
		})
	}
	if _, err := NewFromTemplate("missing"); err == nil {
		t.Fatal("unknown template accepted")
	}
}

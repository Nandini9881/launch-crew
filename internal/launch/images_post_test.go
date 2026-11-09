package launch_test

import (
	"fmt"
	"image/color"
	"reflect"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/launch"
)

func postAssets(in map[string]any) []string {
	var out []string
	assets, _ := in["assets"].([]any)
	for _, a := range assets {
		img, _ := a.(map[string]any)["image"].(map[string]any)
		out = append(out, img["url"].(string))
	}
	return out
}

func (e *env) addImages(t *testing.T, n int) []string {
	t.Helper()
	var urls []string
	for i := 0; i < n; i++ {
		img, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 6, 6, color.RGBA{uint8(i * 20), 0, 0, 255}))
		if err != nil {
			t.Fatalf("image %d: %v", i+1, err)
		}
		urls = append(urls, img.URL)
	}
	return urls
}

func (e *env) captainPostsByChannel(t *testing.T) map[string][]string {
	t.Helper()
	got := map[string][]string{}
	for _, in := range e.createInputs() {
		got[in["channelId"].(string)] = postAssets(in)
	}
	return got
}

// Captain posts carry the card slot then the images, cut to each network:
// X takes 4, LinkedIn takes them all.
func TestLaunchJobAttachesImages(t *testing.T) {
	e := setup(t)
	imgs := e.addImages(t, 4)
	if err := launch.Launch(e.ctx, e.deps, e.launch.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.drain(t); err != nil {
		t.Fatal(err)
	}
	got := e.captainPostsByChannel(t)
	const card = "https://cdn.example/cards/release.png"
	if want := append([]string{card}, imgs[:3]...); !reflect.DeepEqual(got["ch-x"], want) {
		t.Errorf("X assets = %v, want %v", got["ch-x"], want)
	}
	if want := append([]string{card}, imgs...); !reflect.DeepEqual(got["ch-li"], want) {
		t.Errorf("LinkedIn assets = %v, want %v", got["ch-li"], want)
	}
}

func TestLaunchJobCardSwitchOff(t *testing.T) {
	e := setup(t)
	imgs := e.addImages(t, 2)
	if err := e.q.SetLaunchIncludeCard(e.ctx, sqlc.SetLaunchIncludeCardParams{ID: e.launch.ID, IncludeCard: false}); err != nil {
		t.Fatal(err)
	}
	if err := launch.Launch(e.ctx, e.deps, e.launch.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.drain(t); err != nil {
		t.Fatal(err)
	}
	got := e.captainPostsByChannel(t)
	for _, ch := range []string{"ch-x", "ch-li"} {
		if !reflect.DeepEqual(got[ch], imgs) {
			t.Errorf("%s assets = %v, want just the images %v", ch, got[ch], imgs)
		}
	}
}

func TestLaunchJobNoCardNoImagesSendsNone(t *testing.T) {
	e := setup(t)
	if err := e.q.SetLaunchIncludeCard(e.ctx, sqlc.SetLaunchIncludeCardParams{ID: e.launch.ID, IncludeCard: false}); err != nil {
		t.Fatal(err)
	}
	if err := launch.Launch(e.ctx, e.deps, e.launch.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.drain(t); err != nil {
		t.Fatal(err)
	}
	ins := e.createInputs()
	if len(ins) != 2 {
		t.Fatalf("%d captain posts, want 2", len(ins))
	}
	for _, in := range ins {
		if _, has := in["assets"]; has {
			t.Errorf("%s sent assets %v with the card off and no images", in["channelId"], in["assets"])
		}
	}
	_ = fmt.Sprint
}

package crew_test

import (
	"reflect"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/crew"
)

func imgs(n int) []crew.Image {
	out := make([]crew.Image, n)
	for i := range out {
		out[i] = crew.Image{URL: string(rune('a' + i)), Bytes: 500_000}
	}
	return out
}

func TestAttachmentsOrderAndCap(t *testing.T) {
	got := crew.Attachments("card", true, imgs(5), 4, 0)
	if want := []string{"card", "a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := crew.Attachments("card", true, imgs(5), 20, 0); len(got) != 6 || got[0] != "card" || got[5] != "e" {
		t.Errorf("uncapped = %v", got)
	}
}

func TestAttachmentsNoCard(t *testing.T) {
	if got := crew.Attachments("card", false, imgs(5), 4, 0); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("card off: %v", got)
	}
	if got := crew.Attachments("", true, imgs(5), 4, 0); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("no card to include: %v", got)
	}
	if got := crew.Attachments("", false, nil, 4, 0); got != nil {
		t.Errorf("nothing to attach: %v", got)
	}
}

func TestAttachmentsMaxZero(t *testing.T) {
	if got := crew.Attachments("card", true, imgs(1), 0, 0); got != nil {
		t.Errorf("max 0 should attach nothing, got %v", got)
	}
	if got := crew.Attachments("card", true, imgs(1), -1, 0); got != nil {
		t.Errorf("negative max should attach nothing, got %v", got)
	}
}

// A network with a byte limit (Bluesky, 1 MB) gets the small copy of an image
// over it, and skips one that has no copy; the cap counts what is sent.
func TestAttachmentsByteLimitUsesSmallCopies(t *testing.T) {
	images := []crew.Image{
		{URL: "a", Bytes: 500_000},
		{URL: "b", Bytes: 3_000_000, Small: "b-small"},
		{URL: "c", Bytes: 2_000_000},
		{URL: "d", Bytes: 900_000},
		{URL: "e", Bytes: 100_000},
	}
	got := crew.Attachments("card", true, images, 4, 1_000_000)
	if want := []string{"card", "a", "b-small", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("bluesky: got %v, want %v", got, want)
	}
	if got := crew.Attachments("card", true, images, 4, 0); !reflect.DeepEqual(got, []string{"card", "a", "b", "c"}) {
		t.Errorf("no byte limit: got %v", got)
	}
}

package crew_test

import (
	"reflect"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/crew"
)

func TestAttachmentsOrderAndCap(t *testing.T) {
	imgs := []string{"i1", "i2", "i3", "i4", "i5"}
	got := crew.Attachments("card", true, imgs, 4)
	if want := []string{"card", "i1", "i2", "i3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := crew.Attachments("card", true, imgs, 20); len(got) != 6 || got[0] != "card" || got[5] != "i5" {
		t.Errorf("uncapped = %v", got)
	}
}

func TestAttachmentsNoCard(t *testing.T) {
	imgs := []string{"i1", "i2", "i3", "i4", "i5"}
	if got := crew.Attachments("card", false, imgs, 4); !reflect.DeepEqual(got, []string{"i1", "i2", "i3", "i4"}) {
		t.Errorf("card off: %v", got)
	}
	if got := crew.Attachments("", true, imgs, 4); !reflect.DeepEqual(got, []string{"i1", "i2", "i3", "i4"}) {
		t.Errorf("no card to include: %v", got)
	}
	if got := crew.Attachments("", false, nil, 4); got != nil {
		t.Errorf("nothing to attach: %v", got)
	}
}

func TestAttachmentsMaxZero(t *testing.T) {
	if got := crew.Attachments("card", true, []string{"i1"}, 0); got != nil {
		t.Errorf("max 0 should attach nothing, got %v", got)
	}
	if got := crew.Attachments("card", true, []string{"i1"}, -1); got != nil {
		t.Errorf("negative max should attach nothing, got %v", got)
	}
}

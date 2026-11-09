package cards_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/cards"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/r2fake"
)

func TestPutImageKeyAndType(t *testing.T) {
	r2 := r2fake.New("bucket")
	t.Cleanup(r2.Close)
	u := cards.NewUploader(r2.URL, "bucket", "key", "secret", "https://cdn.example/")

	jpg := []byte("\xff\xd8\xff\xe0 not really a jpeg but bytes are bytes")
	sum := sha256.Sum256(jpg)
	wantSha := hex.EncodeToString(sum[:])

	url, sha, err := u.PutImage(context.Background(), jpg)
	if err != nil {
		t.Fatalf("PutImage: %v", err)
	}
	if sha != wantSha {
		t.Errorf("sha = %s, want %s", sha, wantSha)
	}
	if want := "https://cdn.example/uploads/" + wantSha + ".jpg"; url != want {
		t.Errorf("url = %s, want %s", url, want)
	}
	o, ok := r2.Object("uploads/" + wantSha + ".jpg")
	if !ok {
		t.Fatalf("object not stored at uploads/%s.jpg; %d objects", wantSha, r2.Len())
	}
	if o.ContentType != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", o.ContentType)
	}
	if o.CacheControl != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", o.CacheControl)
	}
	if string(o.Body) != string(jpg) {
		t.Errorf("stored body differs")
	}
}

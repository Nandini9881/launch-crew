package api

import (
	"net/http"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/drafting"
)

// voice saves the edited voice and queues a rewrite of unedited drafts.
func (c *Crew) voice(w http.ResponseWriter, r *http.Request) {
	var in drafting.Profile
	if !readJSON(w, r, &in) {
		return
	}
	line, err := crew.UpdateVoice(r.Context(), c.deps, member(r), in)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"voiceLine": line})
}

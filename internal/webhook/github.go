// Package webhook ingests GitHub webhooks: verify, dedupe, enqueue, acknowledge.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

const maxBody = 1 << 20

// ProcessReleasePayload is the job payload for jobs.ProcessRelease.
type ProcessReleasePayload struct {
	DeliveryID string `json:"delivery_id"`
}

// GitHub handles POST /webhooks/github.
type GitHub struct {
	Secret  []byte
	Q       *sqlc.Queries
	Enqueue func(ctx context.Context, kind jobs.Kind, payload any) error
}

func (g *GitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !g.validSignature(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if deliveryID == "" {
		http.Error(w, "missing delivery id", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	ctx := r.Context()

	_, err = g.Q.InsertWebhookDelivery(ctx, sqlc.InsertWebhookDeliveryParams{
		DeliveryID: deliveryID, Event: event, Payload: body,
	})
	if errors.Is(err, pgx.ErrNoRows) { // duplicate delivery
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err != nil {
		log.Printf("webhook: insert delivery %s: %v", deliveryID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if event == "release" {
		var p struct {
			Action string `json:"action"`
		}
		if json.Unmarshal(body, &p) == nil && p.Action == "published" {
			if err := g.Enqueue(ctx, jobs.ProcessRelease, ProcessReleasePayload{DeliveryID: deliveryID}); err != nil {
				log.Printf("webhook: enqueue %s: %v", deliveryID, err)
				// Drop the row so GitHub's retry is not treated as a duplicate.
				if derr := g.Q.DeleteWebhookDelivery(context.WithoutCancel(ctx), deliveryID); derr != nil {
					log.Printf("webhook: delete delivery %s: %v", deliveryID, derr)
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (g *GitHub) validSignature(header string, body []byte) bool {
	if len(g.Secret) == 0 {
		return false // fail closed
	}
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, g.Secret)
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

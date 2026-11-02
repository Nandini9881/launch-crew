package drafting

import (
	"context"
	"time"
)

const (
	defaultBudget    = 8 * time.Second
	defaultThreshold = 0.35
	maxOverlapRetry  = 2
)

// Service picks model or template drafts and keeps a crew's drafts distinct.
type Service struct {
	Model     Drafter
	Template  Drafter
	Budget    time.Duration // covers one DraftFor call, default 8s
	Threshold float64       // overlap above this regenerates, default 0.35
}

// Result is the outcome of DraftFor.
type Result struct {
	Draft        Draft
	Overlap      float64
	Flagged      bool
	UsedTemplate bool
}

func maxOverlap(text string, recent []string) float64 {
	var m float64
	for _, r := range recent {
		if o := TrigramOverlap(text, r); o > m {
			m = o
		}
	}
	return m
}

// DraftFor drafts one post. It never fails because of the model: errors,
// refusals and timeouts fall back to the template. Model calls are at most
// 6 (initial, lint retry, then two overlap regenerations each with a lint
// retry), all inside one Budget.
func (s *Service) DraftFor(ctx context.Context, req DraftRequest, recent []string) (Result, error) {
	tmpl, budget, threshold := s.Template, s.Budget, s.Threshold
	if tmpl == nil {
		tmpl = Template{}
	}
	if budget <= 0 {
		budget = defaultBudget
	}
	if threshold <= 0 {
		threshold = defaultThreshold
	}

	td, err := tmpl.Draft(ctx, req)
	if err != nil {
		return Result{}, err
	}
	fallback := func() Result {
		td.Hook = req.Hook
		if td.Hook == "" {
			td.Hook = AssignHook(nil)
		}
		d := FitToRules(td, req.Rules)
		return Result{Draft: d, Overlap: maxOverlap(d.Text, recent), UsedTemplate: true}
	}
	if !req.AllowModel || s.Model == nil {
		return fallback(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	// gen drafts with one hook, regenerating once if the text has tells.
	gen := func(hook HookType) (Draft, error) {
		r := req
		r.Hook = hook
		d, err := s.Model.Draft(ctx, r)
		if err != nil {
			return Draft{}, err
		}
		if tells := Lint(d.Text); len(tells) > 0 {
			r.Avoid = append([]string(nil), req.Avoid...)
			for _, t := range tells {
				r.Avoid = append(r.Avoid, "do not: "+t)
			}
			d, err = s.Model.Draft(ctx, r)
			if err != nil {
				return Draft{}, err
			}
		}
		d.Hook = hook
		return FitToRules(d, req.Rules), nil
	}

	hook := req.Hook
	if hook == "" {
		hook = AssignHook(nil)
	}
	tried := []HookType{hook}
	d, err := gen(hook)
	if err != nil {
		return fallback(), nil
	}
	ov := maxOverlap(d.Text, recent)
	for i := 0; i < maxOverlapRetry && ov > threshold; i++ {
		hook = AssignHook(tried)
		tried = append(tried, hook)
		if d, err = gen(hook); err != nil {
			return fallback(), nil
		}
		ov = maxOverlap(d.Text, recent)
	}
	return Result{Draft: d, Overlap: ov, Flagged: ov > threshold}, nil
}

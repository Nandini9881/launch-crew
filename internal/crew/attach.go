package crew

// Attachments is what a post carries, in order: the card slot (the hero or
// a card) when includeCard is on and card is set, then the launch's images,
// cut to max, the network's images-per-post limit. A max of zero or less
// means the network takes no images.
func Attachments(card string, includeCard bool, images []string, max int) []string {
	if max <= 0 {
		return nil
	}
	var out []string
	if includeCard && card != "" {
		out = append(out, card)
	}
	for _, u := range images {
		if len(out) >= max {
			break
		}
		out = append(out, u)
	}
	if len(out) > max {
		out = out[:max]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Package crew posts crew members' drafts to their own Buffer queues.
package crew

import (
	"math/rand/v2"
	"time"
)

// Timing is when a crew post goes out.
type Timing string

const (
	TimingWindow Timing = "window" // customScheduled inside the launch window
	TimingQueue  Timing = "queue"  // next free slot
	TimingDraft  Timing = "draft"  // saved as a Buffer draft
)

const minOffset = 5 // minutes after launch_at

// DueAt returns launchAt plus a uniform offset in [5, windowMin] minutes,
// in whole seconds. A window under 5 minutes is treated as 5.
func DueAt(launchAt time.Time, windowMin int, rnd *rand.Rand) time.Time {
	if windowMin < minOffset {
		windowMin = minOffset
	}
	span := int64(windowMin-minOffset) * 60
	return launchAt.Add(time.Duration(minOffset*60+rnd.Int64N(span+1)) * time.Second)
}

// minLead keeps a window post at least this far after the tap.
const minLead = 2 * time.Minute

// DueAtFrom is DueAt floored at now+2m: a late tap draws uniformly from
// what is left of the window. ok is false when less than 2 minutes remain.
func DueAtFrom(launchAt time.Time, windowMin int, now time.Time, rnd *rand.Rand) (time.Time, bool) {
	if windowMin < minOffset {
		windowMin = minOffset
	}
	lo := now.Add(minLead).Truncate(time.Second).Add(time.Second)
	end := launchAt.Add(time.Duration(windowMin) * time.Minute)
	if lo.After(end) {
		return time.Time{}, false
	}
	if first := launchAt.Add(minOffset * time.Minute); !lo.After(first) {
		return DueAt(launchAt, windowMin, rnd), true
	}
	span := int64(end.Sub(lo) / time.Second)
	return lo.Add(time.Duration(rnd.Int64N(span+1)) * time.Second), true
}

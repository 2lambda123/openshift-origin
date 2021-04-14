package monitor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// Monitor records events that have occurred in memory and can also periodically
// sample results.
type Monitor struct {
	interval            time.Duration
	samplers            []SamplerFunc
	intervalCreationFns []IntervalCreationFunc

	lock           sync.Mutex
	events         []*monitorapi.Event
	samples        []*sample
	unsortedEvents []*monitorapi.Event
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor() *Monitor {
	return NewMonitorWithInterval(15 * time.Second)
}

// NewMonitorWithInterval creates a monitor that samples at the provided
// interval.
func NewMonitorWithInterval(interval time.Duration) *Monitor {
	return &Monitor{
		interval: interval,
	}
}

var _ Interface = &Monitor{}

// StartSampling starts sampling every interval until the provided context is done.
// A sample is captured when the context is closed.
func (m *Monitor) StartSampling(ctx context.Context) {
	if m.interval == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		hasConditions := false
		for {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				hasConditions = m.sample(hasConditions)
				return
			}
			hasConditions = m.sample(hasConditions)
		}
	}()
}

// AddSampler adds a sampler function to the list of samplers to run every interval.
// Conditions discovered this way are recorded with a start and end time if they persist
// across multiple sampling intervals.
func (m *Monitor) AddSampler(fn SamplerFunc) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.samplers = append(m.samplers, fn)
}

// Record captures one or more conditions at the current time. All conditions are recorded
// in monotonic order as Event objects.
func (m *Monitor) Record(conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	t := time.Now().UTC()
	for _, condition := range conditions {
		m.events = append(m.events, &monitorapi.Event{
			At:        t,
			Condition: condition,
		})
	}
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as Event objects.
func (m *Monitor) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, condition := range conditions {
		m.unsortedEvents = append(m.unsortedEvents, &monitorapi.Event{
			At:        t,
			Condition: condition,
		})
	}
}

func (m *Monitor) sample(hasPrevious bool) bool {
	m.lock.Lock()
	samplers := m.samplers
	m.lock.Unlock()

	now := time.Now().UTC()
	var conditions []*monitorapi.Condition
	for _, fn := range samplers {
		conditions = append(conditions, fn(now)...)
	}
	if len(conditions) == 0 {
		if !hasPrevious {
			return false
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	t := time.Now().UTC()
	m.samples = append(m.samples, &sample{
		at:         t,
		conditions: conditions,
	})
	return len(conditions) > 0
}

func (m *Monitor) snapshot() ([]*sample, []*monitorapi.Event, monitorapi.Events) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.samples, m.events, m.unsortedEvents
}

// Conditions returns all conditions that were sampled in the interval
// between from and to. If that does not include a sample interval, no
// results will be returned. EventIntervals are returned in order of
// their first sampling. A condition that was only sampled once is
// returned with from == to. No duplicate conditions are returned
// unless a sampling interval did not report that value.
func (m *Monitor) Conditions(from, to time.Time) monitorapi.EventIntervals {
	samples, _, _ := m.snapshot()
	return filterSamples(samples, from, to)
}

// Events returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// EventIntervals are returned in order of their occurrence.
func (m *Monitor) EventIntervals(from, to time.Time) monitorapi.EventIntervals {
	samples, events, unsortedEvents := m.snapshot()
	intervals := filterSamples(samples, from, to)
	events = mergeEvents(filterEvents(events, from, to), filterAndSortEvents(unsortedEvents, from, to))

	// create additional intervals from events
	for _, createIntervals := range m.intervalCreationFns {
		intervals = append(intervals, createIntervals(events, from, to)...)
	}

	// merge the two sets of inputs
	mustSort := len(intervals) > 0
	for i := range events {
		if i > 0 && events[i-1].At.After(events[i].At) {
			fmt.Printf("ERROR: event %d out of order\n  %#v\n  %#v\n", i, events[i-1], events[i])
		}

		to := events[i].At
		from := events[i].At
		if from.IsZero() {
			from = to
		}

		condition := events[i].Condition
		intervals = append(intervals, monitorapi.EventInterval{
			From:      from,
			To:        to,
			Condition: condition,
		})
	}
	if mustSort {
		sort.Sort(intervals)
	}
	return intervals
}

func filterSamples(samples []*sample, from, to time.Time) monitorapi.EventIntervals {
	if len(samples) == 0 {
		return nil
	}

	if !from.IsZero() {
		first := sort.Search(len(samples), func(i int) bool {
			return samples[i].at.After(from)
		})
		if first == -1 {
			return nil
		}
		samples = samples[first:]
	}

	if !to.IsZero() {
		for i, sample := range samples {
			if sample.at.After(to) {
				samples = samples[:i]
				break
			}
		}
	}
	if len(samples) == 0 {
		return nil
	}

	intervals := make(monitorapi.EventIntervals, 0, len(samples)*2)
	activeConditions := map[monitorapi.Condition]*monitorapi.EventInterval{}
	for _, sample := range samples {
		nextActionConditions := map[monitorapi.Condition]*monitorapi.EventInterval{}

		for _, condition := range sample.conditions {
			// if a condition was previously active and is still active now, then we need to adjust the .To to reflect
			// that the condition is ongoing and we need to ensure we track it for active conditions next sample.
			if existingInterval, conditionIsPreviouslyActive := activeConditions[*condition]; conditionIsPreviouslyActive {
				// this mutation is actually modifying the pointer value already present in the returned intervals slice.
				existingInterval.To = sample.at
				nextActionConditions[*condition] = existingInterval
				continue
			}
			intervals = append(intervals, monitorapi.EventInterval{
				Condition: *condition,
				From:      sample.at,
				To:        sample.at,
			})
			nextActionConditions[*condition] = &intervals[len(intervals)-1]
		}

		activeConditions = nextActionConditions
	}
	return intervals
}

func filterEvents(events []*monitorapi.Event, from, to time.Time) []*monitorapi.Event {
	if from.IsZero() && to.IsZero() {
		return events
	}

	first := sort.Search(len(events), func(i int) bool {
		return events[i].At.After(from)
	})
	if first == -1 {
		return nil
	}
	if to.IsZero() {
		return events[first:]
	}
	for i := first; i < len(events); i++ {
		if events[i].At.After(to) {
			return events[first:i]
		}
	}
	return events[first:]
}

// mergeEvents returns a sorted list of all events provided as sources. This could be
// more efficient by requiring all sources to be sorted (would be O(n)).
func mergeEvents(events ...[]*monitorapi.Event) []*monitorapi.Event {
	total := 0
	for _, event := range events {
		total += len(event)
	}
	merged := make([]*monitorapi.Event, 0, total)
	for _, event := range events {
		merged = append(merged, event...)
	}
	sort.Sort(monitorapi.Events(merged))
	return merged
}

// filterAndSortEvents returns events before to and after from in sorted order, assuming
// the input events are unsorted.
func filterAndSortEvents(events monitorapi.Events, from, to time.Time) monitorapi.Events {
	copied := make(monitorapi.Events, 0, len(events))

	if from.IsZero() && to.IsZero() {
		for _, e := range events {
			copied = append(copied, e)
		}
		sort.Sort(monitorapi.Events(copied))
		return copied
	}

	for _, e := range events {
		if !e.At.After(from) {
			continue
		}
		if !to.IsZero() && !e.At.Before(to) {
			continue
		}
		copied = append(copied, e)
	}
	sort.Sort(monitorapi.Events(copied))
	return copied
}

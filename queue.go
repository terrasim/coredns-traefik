package traefik_coredns_plugin

import (
	"time"
)

type IncCacheQueue[K comparable] struct {
	entries         map[K][]time.Time
	entriesFullChan map[K]chan bool

	cacheDuration     time.Duration
	cacheSize         uint
	cacheFullDuration time.Duration
}

// Inc increments the internal counter. It returns if the queue was filled with this increment. If the queue not full or
// was already full before this increment, it will return false.
func (icq *IncCacheQueue[K]) Inc(key K) bool {
	values, ok := icq.entries[key]
	now := time.Now()
	if !ok {
		values = []time.Time{}
		icq.entriesFullChan[key] = make(chan bool)
	}
	fullBefore := icq.Full(key)
	icq.entries[key] = append(values, now)
	fullAfter := icq.Full(key)

	if !ok || uint(len(values)) == 0 {
		go func() {
			for {
				values := icq.entries[key]
				if len(values) == 0 {
					break
				}
				select {
				case <-time.After(values[0].Add(icq.cacheDuration).Sub(now)):
					icq.entries[key] = icq.entries[key][1:]
					continue
				case <-icq.entriesFullChan[key]:
					time.Sleep(icq.cacheFullDuration)
					icq.entries[key] = []time.Time{}
					icq.entriesFullChan[key] = make(chan bool)
				}
			}
		}()
	}

	if fullBefore != fullAfter {
		icq.entriesFullChan[key] <- true
	}

	return fullBefore != fullAfter
}

// Full returns if the queue is full.
func (icq *IncCacheQueue[K]) Full(key K) bool {
	values, ok := icq.entries[key]
	if !ok || uint(len(values)) < icq.cacheSize {
		return false
	}
	return true
}

func NewQueue[K comparable](cacheDuration time.Duration, cacheSize uint, cacheFullDuration time.Duration) *IncCacheQueue[K] {
	return &IncCacheQueue[K]{
		entries:           map[K][]time.Time{},
		entriesFullChan:   map[K]chan bool{},
		cacheDuration:     cacheDuration,
		cacheSize:         cacheSize,
		cacheFullDuration: cacheFullDuration,
	}
}

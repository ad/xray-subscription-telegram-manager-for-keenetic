package telegram

import (
	"context"
	"sync"
	"time"
)

type RateLimiter struct {
	requests map[int64][]time.Time
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[int64][]time.Time),
		mutex:    sync.RWMutex{},
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) IsAllowed(userID int64) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()

	userRequests := rl.requests[userID]

	var validRequests []time.Time
	for _, reqTime := range userRequests {
		if now.Sub(reqTime) < rl.window {
			validRequests = append(validRequests, reqTime)
		}
	}

	if len(validRequests) >= rl.limit {
		return false
	}

	validRequests = append(validRequests, now)
	rl.requests[userID] = validRequests

	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	for userID, requests := range rl.requests {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if now.Sub(reqTime) < rl.window {
				validRequests = append(validRequests, reqTime)
			}
		}

		if len(validRequests) == 0 {
			delete(rl.requests, userID)
		} else {
			rl.requests[userID] = validRequests
		}
	}
}

func (rl *RateLimiter) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.Cleanup()
		}
	}
}

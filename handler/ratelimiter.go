package handler

import (
    "golang.org/x/time/rate"
    "sync"
)

type RateLimiter struct {
    limiter *rate.Limiter
    mu      sync.Mutex
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
    var limiter *rate.Limiter
    if rps > 0 {
        limiter = rate.NewLimiter(rate.Limit(rps), burst)
    }
    return &RateLimiter{
        limiter: limiter,
    }
}

func (r *RateLimiter) Allow() bool {
    if r.limiter == nil {
        return true
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.limiter.Allow()
}
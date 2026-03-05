// Package ratelimit provides a token-bucket rate limiter for gRPC RPCs.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Limiter is a simple token-bucket rate limiter.
type Limiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64 // tokens per second
	last     time.Time
	methods  map[string]bool
}

// New creates a Limiter that allows burst requests and refills at rate
// tokens/second. Only RPCs whose full method name is in methods are limited.
func New(burst int, rate float64, methods []string) *Limiter {
	m := make(map[string]bool, len(methods))
	for _, method := range methods {
		m[method] = true
	}
	return &Limiter{
		tokens:  float64(burst),
		max:     float64(burst),
		rate:    rate,
		last:    time.Now(),
		methods: m,
	}
}

// Allow consumes one token. Returns false if the bucket is empty.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.last = now

	l.tokens += elapsed * l.rate
	if l.tokens > l.max {
		l.tokens = l.max
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// UnaryInterceptor returns a gRPC interceptor that rejects limited RPCs
// with ResourceExhausted when the bucket is empty.
func (l *Limiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if l.methods[info.FullMethod] && !l.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded, try again later")
		}
		return handler(ctx, req)
	}
}

package ratelimit

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAllow_BurstThenReject(t *testing.T) {
	t.Parallel()
	l := New(3, 1, []string{"/test"})

	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Fatalf("Allow() = false on call %d, want true", i)
		}
	}
	if l.Allow() {
		t.Error("Allow() = true after burst exhausted, want false")
	}
}

func TestInterceptor_LimitedMethod(t *testing.T) {
	t.Parallel()
	l := New(1, 0, []string{"/hermes.HermesService/Notify"})
	interceptor := l.UnaryInterceptor()

	handler := func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/hermes.HermesService/Notify"}

	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}

	_, err = interceptor(context.Background(), nil, info, handler)
	if s, ok := status.FromError(err); !ok || s.Code() != codes.ResourceExhausted {
		t.Errorf("second call: expected ResourceExhausted, got %v", err)
	}
}

func TestInterceptor_UnlimitedMethod(t *testing.T) {
	t.Parallel()
	l := New(1, 0, []string{"/hermes.HermesService/Notify"})
	interceptor := l.UnaryInterceptor()

	handler := func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/hermes.HermesService/List"}

	for i := 0; i < 10; i++ {
		_, err := interceptor(context.Background(), nil, info, handler)
		if err != nil {
			t.Fatalf("call %d: unexpected error %v", i, err)
		}
	}
}

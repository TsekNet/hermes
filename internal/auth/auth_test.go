package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGenerateAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(dir, "session.token") }
	t.Cleanup(func() { TokenPath = orig })

	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token) != tokenBytes*2 {
		t.Errorf("token length = %d, want %d hex chars", len(token), tokenBytes*2)
	}

	loaded, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if loaded != token {
		t.Errorf("loaded token = %q, want %q", loaded, token)
	}

	info, err := os.Stat(filepath.Join(dir, "session.token"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("token file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestRemoveToken(t *testing.T) {
	dir := t.TempDir()
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(dir, "session.token") }
	t.Cleanup(func() { TokenPath = orig })

	GenerateToken()
	RemoveToken()

	if _, err := os.Stat(filepath.Join(dir, "session.token")); !os.IsNotExist(err) {
		t.Error("token file should be removed")
	}
}

func TestLoadToken_Missing(t *testing.T) {
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(t.TempDir(), "nonexistent") }
	t.Cleanup(func() { TokenPath = orig })

	_, err := LoadToken()
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestUnaryInterceptor_ValidToken(t *testing.T) {
	t.Parallel()
	interceptor := UnaryInterceptor("test-token-123")

	md := metadata.New(map[string]string{metadataKey: "test-token-123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/hermes.HermesService/Notify"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler not called")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
}

func TestUnaryInterceptor_MissingMetadata(t *testing.T) {
	t.Parallel()
	interceptor := UnaryInterceptor("token")

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestUnaryInterceptor_WrongToken(t *testing.T) {
	t.Parallel()
	interceptor := UnaryInterceptor("correct-token")

	md := metadata.New(map[string]string{metadataKey: "wrong-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

func TestUnaryInterceptor_EmptyAuthHeader(t *testing.T) {
	t.Parallel()
	interceptor := UnaryInterceptor("token")

	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestPerRPCCredentials(t *testing.T) {
	t.Parallel()
	creds := &PerRPCCredentials{Token: "my-token"}

	md, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata: %v", err)
	}
	if md[metadataKey] != "my-token" {
		t.Errorf("token = %q, want my-token", md[metadataKey])
	}
	if creds.RequireTransportSecurity() {
		t.Error("should not require transport security (localhost only)")
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	dir := t.TempDir()
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(dir, "session.token") }
	t.Cleanup(func() { TokenPath = orig })

	t1, _ := GenerateToken()
	t2, _ := GenerateToken()
	if t1 == t2 {
		t.Error("two generated tokens should differ")
	}
}

func TestGenerateToken_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(dir, "session.token") }
	t.Cleanup(func() { TokenPath = orig })

	_, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}

func TestRemoveToken_Idempotent(t *testing.T) {
	dir := t.TempDir()
	orig := TokenPath
	TokenPath = func() string { return filepath.Join(dir, "session.token") }
	t.Cleanup(func() { TokenPath = orig })

	RemoveToken()
	RemoveToken()
}

func TestLoadToken_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.token")
	orig := TokenPath
	TokenPath = func() string { return path }
	t.Cleanup(func() { TokenPath = orig })

	os.WriteFile(path, []byte("  abc123  \n"), 0600)
	got, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got != "abc123" {
		t.Errorf("LoadToken = %q, want 'abc123'", got)
	}
}

func TestTokenPath_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()
	p := tokenPath()
	if p == "" {
		t.Error("tokenPath() returned empty string")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("tokenPath() = %q, want absolute path", p)
	}
}

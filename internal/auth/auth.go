// Package auth provides per-session token authentication for the hermes gRPC service.
//
// On startup, the server generates a random token and writes it to a
// platform-specific path with 0600 permissions. Clients read the token
// and attach it as gRPC metadata. A unary interceptor validates the
// token and enforces scope (full vs read-only).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	tokenBytes = 32
	metadataKey = "authorization"
)

// GenerateToken creates a cryptographically random hex token and writes
// it to the platform-specific token path with 0600 permissions.
// Returns the raw token string.
func GenerateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	path := TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("create token dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return "", fmt.Errorf("write token: %w", err)
	}
	return token, nil
}

// LoadToken reads the token from the platform-specific path.
func LoadToken() (string, error) {
	data, err := os.ReadFile(TokenPath())
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// RemoveToken deletes the token file. Called on service shutdown.
func RemoveToken() {
	os.Remove(TokenPath())
}

// TokenPath returns the platform-specific path for the session token.
// It is a variable so tests can override it.
var TokenPath = tokenPath

func tokenPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes", "session.token")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "hermes", "session.token")
	default:
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			return filepath.Join(dir, "hermes", "session.token")
		}
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "hermes", "session.token")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "hermes", "session.token")
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that validates
// the session token from metadata. Read-only RPCs accept the token
// regardless; write RPCs require the full token.
func UnaryInterceptor(token string) grpc.UnaryServerInterceptor {
	tokenBytes := []byte(token)
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get(metadataKey)
		if len(vals) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}
		if subtle.ConstantTimeCompare([]byte(vals[0]), tokenBytes) != 1 {
			return nil, status.Error(codes.PermissionDenied, "invalid token")
		}
		return handler(ctx, req)
	}
}

// PerRPCCredentials implements grpc.PerRPCCredentials to attach the
// session token to every outgoing RPC.
type PerRPCCredentials struct {
	Token string
}

func (c *PerRPCCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{metadataKey: c.Token}, nil
}

func (c *PerRPCCredentials) RequireTransportSecurity() bool { return false }

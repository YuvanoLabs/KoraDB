package server

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	"KoraDB/internal/storage"
)

func TestAuthInterceptorAllowsUnauthenticatedHealthCheck(t *testing.T) {
	st, err := storage.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	called := false
	_, err = AuthInterceptor(st)(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: healthCheckMethod}, func(context.Context, any) (any, error) {
		called = true
		return "ok", nil
	})
	if err != nil || !called {
		t.Fatalf("health handler called=%v err=%v, want true nil", called, err)
	}
}

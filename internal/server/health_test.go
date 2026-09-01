package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/YuvanoLabs/KoraDB/internal/storage"
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

func TestBearerTokenRequiresExactlyOneBearerValue(t *testing.T) {
	valid := "kdb_0011223344556677_" + repeatTokenByte('a', 64)
	cases := []struct {
		name string
		vals []string
		want string
	}{
		{name: "valid", vals: []string{"Bearer " + valid}, want: valid},
		{name: "raw token", vals: []string{valid}},
		{name: "empty bearer", vals: []string{"Bearer "}},
		{name: "multiple values", vals: []string{"Bearer " + valid, "Bearer " + valid}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"authorization": tc.vals})
			got, err := bearerToken(ctx)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("bearerToken(%v) succeeded with %q", tc.vals, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("bearerToken(%v) = %q, %v; want %q, nil", tc.vals, got, err, tc.want)
			}
		})
	}
}

func repeatTokenByte(b byte, count int) string {
	value := make([]byte, count)
	for i := range value {
		value[i] = b
	}
	return string(value)
}

func TestAuditRecordIsStructuredJSON(t *testing.T) {
	record := auditRecord{
		Timestamp:  "2026-08-31T00:00:00Z",
		Method:     "Query",
		Principal:  "reader/readonly",
		Peer:       "127.0.0.1:1234",
		Code:       "OK",
		DurationMs: 12,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("audit JSON did not round-trip: %v", err)
	}
	if decoded["method"] != "Query" || decoded["principal"] != "reader/readonly" || decoded["duration_ms"] != float64(12) {
		t.Fatalf("unexpected audit fields: %#v", decoded)
	}
}

func TestDeadlineInterceptorBoundsHandler(t *testing.T) {
	_, err := DeadlineInterceptor(time.Millisecond)(context.Background(), nil, nil, func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return "late", nil
	})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("deadline error code = %s, want DeadlineExceeded (err=%v)", status.Code(err), err)
	}
}

func TestConcurrencyInterceptorRejectsOverload(t *testing.T) {
	interceptor := ConcurrencyInterceptor(1)
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := interceptor(context.Background(), nil, nil, func(context.Context, any) (any, error) {
			close(entered)
			<-release
			return "ok", nil
		})
		finished <- err
	}()
	<-entered
	_, err := interceptor(context.Background(), nil, nil, func(context.Context, any) (any, error) { return "ok", nil })
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("overload code = %s, want ResourceExhausted (err=%v)", status.Code(err), err)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatalf("first request = %v, want nil", err)
	}
}

func TestRateLimitInterceptorUsesBurstAndRefills(t *testing.T) {
	interceptor := RateLimitInterceptor(1000, 1)
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	if _, err := interceptor(context.Background(), nil, nil, handler); err != nil {
		t.Fatalf("first request = %v", err)
	}
	if _, err := interceptor(context.Background(), nil, nil, handler); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("immediate second request code = %s, want ResourceExhausted", status.Code(err))
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := interceptor(context.Background(), nil, nil, handler); err != nil {
		t.Fatalf("refilled request = %v", err)
	}
}

func TestMetricsHTTPHandlerExportsSafeRequestLabels(t *testing.T) {
	metrics := NewMetrics()
	interceptor := AuditInterceptor(metrics)
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/yuvanolabs.koradb.v1.KoraDB/Get"}, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	metrics.HTTPHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics HTTP status = %d, want 200", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `koradb_requests_total{method="Get",code="OK"} 1`) {
		t.Fatalf("metrics missing completed request: %s", body)
	}
	if strings.Contains(body, "principal") || strings.Contains(body, "authorization") {
		t.Fatalf("metrics must not expose identities or credentials: %s", body)
	}
}

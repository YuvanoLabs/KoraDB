package server

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/YuvanoLabs/KoraDB/internal/auth"
	"github.com/YuvanoLabs/KoraDB/internal/storage"
)

type principalCtxKey struct{}

// principalHolder is a mutable cell shared from the (outer) audit interceptor
// into the (inner) auth interceptor. Because it is a pointer, the auth
// interceptor can record the authenticated principal into it and the audit
// interceptor sees that value after the handler returns — even though auth runs
// in a child context the audit interceptor never observes directly. This lets
// audit stay outermost (so it also records auth failures) while still naming
// the principal on success.
type principalHolder struct{ p *auth.Principal }

type holderCtxKey struct{}

var auditLogger = log.New(os.Stderr, "", 0)

type auditRecord struct {
	Timestamp  string `json:"timestamp"`
	Method     string `json:"method"`
	Principal  string `json:"principal"`
	Peer       string `json:"peer"`
	Code       string `json:"code"`
	DurationMs int64  `json:"duration_ms"`
}

// DeadlineInterceptor supplies a server-side upper bound for unary requests.
// A client deadline that expires sooner is preserved. Handlers receive the
// derived context, and a response completed after the deadline is rejected so
// callers never observe work as successful once its request window has ended.
func DeadlineInterceptor(maxDuration time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if maxDuration <= 0 {
			return nil, status.Error(codes.Internal, "server request deadline is misconfigured")
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= maxDuration {
			if err := ctx.Err(); err != nil {
				return nil, status.FromContextError(err).Err()
			}
			return handler(ctx, req)
		}

		requestCtx, cancel := context.WithTimeout(ctx, maxDuration)
		defer cancel()
		response, err := handler(requestCtx, req)
		if contextErr := requestCtx.Err(); contextErr != nil {
			return nil, status.FromContextError(contextErr).Err()
		}
		return response, err
	}
}

// ConcurrencyInterceptor rejects excess unary requests instead of allowing an
// unbounded queue to consume server memory. The semaphore is shared by every
// RPC, including the public health endpoint, so its stated capacity reflects
// all work accepted by the process.
func ConcurrencyInterceptor(maxInFlight int) grpc.UnaryServerInterceptor {
	if maxInFlight <= 0 {
		return func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
			return nil, status.Error(codes.Internal, "server concurrency limit is misconfigured")
		}
	}
	semaphore := make(chan struct{}, maxInFlight)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return handler(ctx, req)
		default:
			return nil, status.Error(codes.ResourceExhausted, "server is at its concurrent-request limit")
		}
	}
}

// RateLimitInterceptor applies a shared token-bucket limit to unary requests.
// KoraDB is a single-node service without tenants, so this is deliberately a
// process-wide overload guard rather than a claim of per-tenant quota
// isolation. A non-positive rate disables the guard for development.
func RateLimitInterceptor(requestsPerSecond, burst int) grpc.UnaryServerInterceptor {
	bucket := newTokenBucket(requestsPerSecond, burst)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !bucket.allow(time.Now()) {
			return nil, status.Error(codes.ResourceExhausted, "server request rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

type tokenBucket struct {
	mu        sync.Mutex
	rate      float64
	burst     float64
	tokens    float64
	refreshed time.Time
}

func newTokenBucket(requestsPerSecond, burst int) *tokenBucket {
	if requestsPerSecond <= 0 {
		return &tokenBucket{}
	}
	if burst <= 0 {
		burst = requestsPerSecond
	}
	return &tokenBucket{
		rate:      float64(requestsPerSecond),
		burst:     float64(burst),
		tokens:    float64(burst),
		refreshed: time.Now(),
	}
}

func (b *tokenBucket) allow(now time.Time) bool {
	if b.rate <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = minFloat(b.burst, b.tokens+now.Sub(b.refreshed).Seconds()*b.rate)
	b.refreshed = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// AuthInterceptor authenticates the bearer token and enforces the RBAC policy
// before any handler runs. It is fail-closed: a missing/invalid token yields
// Unauthenticated, and an authenticated caller without sufficient role (or an
// unmapped method) yields PermissionDenied.
func AuthInterceptor(store *storage.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Health is intentionally unauthenticated so orchestrators can check
		// readiness without receiving a database credential. It exposes only a
		// serving status and is registered after the database opens successfully.
		if isPublicHealthMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		token, err := bearerToken(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "missing or malformed authorization token")
		}
		principal, err := auth.Authenticate(store, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		if !principal.Can(info.FullMethod) {
			// Record the identity so the audit log names who was denied.
			if h, ok := ctx.Value(holderCtxKey{}).(*principalHolder); ok {
				h.p = principal
			}
			return nil, status.Errorf(codes.PermissionDenied,
				"role %s may not call %s", principal.Role, shortMethod(info.FullMethod))
		}
		if h, ok := ctx.Value(holderCtxKey{}).(*principalHolder); ok {
			h.p = principal
		}
		ctx = context.WithValue(ctx, principalCtxKey{}, principal)
		return handler(ctx, req)
	}
}

// AuditInterceptor logs one JSON record per unary request: who, what, outcome,
// peer, and latency. It deliberately logs NO request/response payloads or query
// values, which can contain PII or otherwise-classified data. It records failed
// requests (including auth failures) too. It is the outermost interceptor.
func AuditInterceptor(metrics ...*Metrics) grpc.UnaryServerInterceptor {
	var collector *Metrics
	if len(metrics) > 0 {
		collector = metrics[0]
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		collector.begin()
		holder := &principalHolder{}
		ctx = context.WithValue(ctx, holderCtxKey{}, holder)
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		principal := "anonymous"
		if holder.p != nil {
			principal = holder.p.Name + "/" + holder.p.Role.String()
		}
		code := status.Code(err)
		collector.record(shortMethod(info.FullMethod), code.String(), dur)
		recordAudit(auditRecord{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			Method:     shortMethod(info.FullMethod),
			Principal:  principal,
			Peer:       peerAddr(ctx),
			Code:       code.String(),
			DurationMs: dur.Milliseconds(),
		})
		return resp, err
	}
}

func recordAudit(record auditRecord) {
	encoded, err := json.Marshal(record)
	if err != nil {
		// Every field is a string or an integer, so this should be unreachable.
		// Preserve availability if a future field violates that expectation.
		log.Printf("audit serialization failed: %v", err)
		return
	}
	auditLogger.Print(string(encoded))
}

// PrincipalFromContext returns the authenticated principal, if any.
func PrincipalFromContext(ctx context.Context) (*auth.Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(*auth.Principal)
	return p, ok
}

func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "no metadata")
	}
	vals := md.Get("authorization")
	if len(vals) != 1 {
		return "", status.Error(codes.Unauthenticated, "no authorization")
	}
	v := strings.TrimSpace(vals[0])
	if !strings.HasPrefix(v, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "malformed authorization")
	}
	token := strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "malformed authorization")
	}
	return token, nil
}

func peerAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}

func shortMethod(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

package server

import (
	"context"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"KoraDB/internal/auth"
	"KoraDB/internal/storage"
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

// AuditInterceptor logs one structured record per request: who, what, outcome,
// peer, and latency. It deliberately logs NO request/response payloads or query
// values, which can contain PII or otherwise-classified data. It records failed
// requests (including auth failures) too. It is the outermost interceptor.
func AuditInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		holder := &principalHolder{}
		ctx = context.WithValue(ctx, holderCtxKey{}, holder)
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		principal := "anonymous"
		if holder.p != nil {
			principal = holder.p.Name + "/" + holder.p.Role.String()
		}
		code := status.Code(err)
		log.Printf("audit method=%s principal=%s peer=%s code=%s dur=%s",
			shortMethod(info.FullMethod), principal, peerAddr(ctx), code, dur)
		return resp, err
	}
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
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "no authorization")
	}
	v := vals[0]
	if strings.HasPrefix(v, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(v, "Bearer ")), nil
	}
	return strings.TrimSpace(v), nil
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

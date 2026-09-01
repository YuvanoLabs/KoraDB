package server

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/YuvanoLabs/KoraDB/api/gen/KoraDBv1"
	"github.com/YuvanoLabs/KoraDB/internal/auth"
)

// CreateKey mints a new API key (admin only — enforced by the interceptor). The
// returned token is shown to the caller exactly once.
func (s *Server) CreateKey(ctx context.Context, req *pb.CreateKeyRequest) (*pb.CreateKeyResponse, error) {
	role, err := fromPBRole(req.GetRole())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	var expiresAt time.Time
	if req.GetExpiresAtUnix() != 0 {
		expiresAt = time.Unix(req.GetExpiresAtUnix(), 0).UTC()
	}
	token, keyID, err := auth.CreateKeyWithExpiry(s.db.Store(), req.GetName(), role, expiresAt)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &pb.CreateKeyResponse{KeyId: keyID, Token: token}, nil
}

func (s *Server) ListKeys(ctx context.Context, _ *pb.ListKeysRequest) (*pb.ListKeysResponse, error) {
	recs, err := auth.List(s.db.Store())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &pb.ListKeysResponse{}
	for _, r := range recs {
		resp.Keys = append(resp.Keys, &pb.KeyInfo{
			KeyId:         r.KeyID,
			Name:          r.Name,
			Role:          toPBRole(r.Role),
			CreatedAtUnix: r.CreatedUnix,
			ExpiresAtUnix: r.ExpiresUnix,
		})
	}
	return resp, nil
}

func (s *Server) RevokeKey(ctx context.Context, req *pb.RevokeKeyRequest) (*pb.RevokeKeyResponse, error) {
	if err := auth.Revoke(s.db.Store(), req.GetKeyId()); err != nil {
		if errors.Is(err, auth.ErrLastAdmin) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.RevokeKeyResponse{}, nil
}

func toPBRole(r auth.Role) pb.Role {
	switch r {
	case auth.RoleReadOnly:
		return pb.Role_ROLE_READONLY
	case auth.RoleReadWrite:
		return pb.Role_ROLE_READWRITE
	case auth.RoleAdmin:
		return pb.Role_ROLE_ADMIN
	default:
		return pb.Role_ROLE_UNSPECIFIED
	}
}

func fromPBRole(r pb.Role) (auth.Role, error) {
	switch r {
	case pb.Role_ROLE_READONLY:
		return auth.RoleReadOnly, nil
	case pb.Role_ROLE_READWRITE:
		return auth.RoleReadWrite, nil
	case pb.Role_ROLE_ADMIN:
		return auth.RoleAdmin, nil
	default:
		return auth.RoleNone, status.Error(codes.InvalidArgument, "role must be readonly, readwrite, or admin")
	}
}

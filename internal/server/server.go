// Package server is Phase B of KoraDB: the gRPC service implementation.
//
// It adapts the KoraDB.v1 wire contract onto the embedded engine (Layers 0–3)
// and query executor (Layer 4). Documents cross the wire as JSON; the engine
// stores them as protobuf bytes. The whole thing compiles into the
// KoraDB-server binary with no external runtime dependencies.
package server

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "KoraDB/api/gen/KoraDBv1"
	"KoraDB/internal/engine"
	"KoraDB/internal/query"
)

// Server implements pb.KoraDBServer over an open engine.DB.
type Server struct {
	pb.UnimplementedKoraDBServer
	db *engine.DB
}

// New returns a gRPC service backed by db.
func New(db *engine.DB) *Server { return &Server{db: db} }

func (s *Server) PutSchema(ctx context.Context, req *pb.PutSchemaRequest) (*pb.PutSchemaResponse, error) {
	v, err := s.db.RegisterSchema(ctx, req.GetName(), req.GetProtoSource())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.PutSchemaResponse{Version: int32(v)}, nil
}

func (s *Server) ListSchemas(ctx context.Context, _ *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error) {
	schemas, err := s.db.Registry().ListSchemas()
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &pb.ListSchemasResponse{}
	for _, sc := range schemas {
		resp.Schemas = append(resp.Schemas, &pb.SchemaInfo{Name: sc.Name, Version: int32(sc.Version)})
	}
	return resp, nil
}

func (s *Server) CreateCollection(ctx context.Context, req *pb.CreateCollectionRequest) (*pb.CreateCollectionResponse, error) {
	_, err := s.db.CreateCollection(req.GetName(), req.GetMessageType(), &engine.CollectionOptions{
		KeyField: req.GetKeyField(),
		Indexes:  req.GetIndexes(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CreateCollectionResponse{}, nil
}

func (s *Server) ListCollections(ctx context.Context, _ *pb.ListCollectionsRequest) (*pb.ListCollectionsResponse, error) {
	colls, err := s.db.ListCollections()
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &pb.ListCollectionsResponse{}
	for _, c := range colls {
		resp.Collections = append(resp.Collections, &pb.CollectionInfo{
			Name:        c.Name,
			MessageType: c.MessageType,
			KeyKind:     string(c.KeyKind),
			KeyField:    c.KeyField,
			Indexes:     c.Indexes,
		})
	}
	return resp, nil
}

func (s *Server) Insert(ctx context.Context, req *pb.InsertRequest) (*pb.InsertResponse, error) {
	id, err := s.db.Insert(req.GetCollection(), []byte(req.GetJson()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.InsertResponse{Id: id}, nil
}

func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	j, err := s.db.Get(req.GetCollection(), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.GetResponse{Json: string(j)}, nil
}

func (s *Server) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if err := s.db.Update(req.GetCollection(), req.GetId(), []byte(req.GetJson())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.UpdateResponse{}, nil
}

func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if err := s.db.Delete(req.GetCollection(), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.DeleteResponse{}, nil
}

func (s *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	filter, err := toFilter(req.GetFilter())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	results, err := query.Execute(s.db, req.GetCollection(), filter)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &pb.QueryResponse{}
	for _, r := range results {
		resp.Results = append(resp.Results, &pb.Document{Id: r.ID, Json: string(r.JSON)})
	}
	return resp, nil
}

// toFilter converts the wire filter AST into the query package's AST. A nil
// wire filter means "match all".
func toFilter(f *pb.Filter) (query.Filter, error) {
	if f == nil {
		return nil, nil
	}
	switch node := f.GetNode().(type) {
	case *pb.Filter_Cmp:
		op, err := toOp(node.Cmp.GetOp())
		if err != nil {
			return nil, err
		}
		return query.Cmp{Field: node.Cmp.GetField(), Op: op, Value: node.Cmp.GetValue()}, nil
	case *pb.Filter_AndGroup:
		subs, err := toFilters(node.AndGroup.GetFilters())
		if err != nil {
			return nil, err
		}
		return query.And{Filters: subs}, nil
	case *pb.Filter_OrGroup:
		subs, err := toFilters(node.OrGroup.GetFilters())
		if err != nil {
			return nil, err
		}
		return query.Or{Filters: subs}, nil
	default:
		return nil, nil
	}
}

func toFilters(in []*pb.Filter) ([]query.Filter, error) {
	out := make([]query.Filter, 0, len(in))
	for _, f := range in {
		qf, err := toFilter(f)
		if err != nil {
			return nil, err
		}
		out = append(out, qf)
	}
	return out, nil
}

func toOp(op pb.Op) (query.Op, error) {
	switch op {
	case pb.Op_OP_EQ:
		return query.Eq, nil
	case pb.Op_OP_NE:
		return query.Ne, nil
	case pb.Op_OP_GT:
		return query.Gt, nil
	case pb.Op_OP_GTE:
		return query.Gte, nil
	case pb.Op_OP_LT:
		return query.Lt, nil
	case pb.Op_OP_LTE:
		return query.Lte, nil
	default:
		return 0, errors.New("query: unspecified comparison operator")
	}
}

// toStatus maps engine errors to gRPC status codes so clients get meaningful,
// language-agnostic error categories.
func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, engine.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, engine.ErrExists), errors.Is(err, engine.ErrDuplicateKey):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		// Most remaining engine errors are caused by bad client input (invalid
		// .proto, malformed JSON, unknown field/collection).
		return status.Error(codes.InvalidArgument, err.Error())
	}
}

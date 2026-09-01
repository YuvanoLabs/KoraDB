package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/YuvanoLabs/KoraDB/api/gen/KoraDBv1"
	"github.com/YuvanoLabs/KoraDB/internal/auth"
	"github.com/YuvanoLabs/KoraDB/internal/engine"
	"github.com/YuvanoLabs/KoraDB/internal/query"
)

// backend is the set of operations the CLI needs. It is implemented twice: an
// embedded backend that opens the database file directly, and a remote backend
// that talks to a KoraDB-server over gRPC. The same commands work either way.
type backend interface {
	PutSchema(name, source string) (int, error)
	ListSchemas() ([]schemaRow, error)
	CreateCollection(name, msgType, keyField string, indexes []string) error
	ListCollections() ([]collRow, error)
	Insert(coll, json string) (id string, err error)
	Get(coll, id string) (json string, err error)
	Update(coll, id, json string) error
	Delete(coll, id string) error
	Backup(w io.Writer) error
	Verify() error
	QueryPage(coll, field string, op query.Op, value string, pageSize int, pageToken string) ([]docRow, string, error)
	CreateKey(name, role string, expiresAtUnix int64) (keyID, token string, err error)
	ListKeys() ([]keyRow, error)
	RevokeKey(keyID string) error
	Close() error
}

type schemaRow struct {
	Name    string
	Version int
}
type collRow struct {
	Name        string
	MessageType string
	KeyKind     string
	Indexes     []string
}
type docRow struct {
	ID   string
	JSON string
}
type keyRow struct {
	KeyID       string
	Name        string
	Role        string
	CreatedUnix int64
	ExpiresUnix int64
}

// --- embedded backend (opens the .db file directly) ---

type embeddedBackend struct{ db *engine.DB }

func openEmbedded(path string) (*embeddedBackend, error) {
	db, err := engine.Open(path)
	if err != nil {
		return nil, err
	}
	return &embeddedBackend{db: db}, nil
}

func (e *embeddedBackend) Close() error { return e.db.Close() }

func (e *embeddedBackend) PutSchema(name, source string) (int, error) {
	return e.db.RegisterSchema(context.Background(), name, source)
}

func (e *embeddedBackend) ListSchemas() ([]schemaRow, error) {
	ss, err := e.db.Registry().ListSchemas()
	if err != nil {
		return nil, err
	}
	out := make([]schemaRow, 0, len(ss))
	for _, s := range ss {
		out = append(out, schemaRow{Name: s.Name, Version: s.Version})
	}
	return out, nil
}

func (e *embeddedBackend) CreateCollection(name, msgType, keyField string, indexes []string) error {
	_, err := e.db.CreateCollection(name, msgType, &engine.CollectionOptions{KeyField: keyField, Indexes: indexes})
	return err
}

func (e *embeddedBackend) ListCollections() ([]collRow, error) {
	cs, err := e.db.ListCollections()
	if err != nil {
		return nil, err
	}
	out := make([]collRow, 0, len(cs))
	for _, c := range cs {
		out = append(out, collRow{Name: c.Name, MessageType: c.MessageType, KeyKind: string(c.KeyKind), Indexes: c.Indexes})
	}
	return out, nil
}

func (e *embeddedBackend) Insert(coll, json string) (string, error) {
	return e.db.Insert(coll, []byte(json))
}

func (e *embeddedBackend) Get(coll, id string) (string, error) {
	j, err := e.db.Get(coll, id)
	return string(j), err
}

func (e *embeddedBackend) Update(coll, id, json string) error {
	return e.db.Update(coll, id, []byte(json))
}

func (e *embeddedBackend) Delete(coll, id string) error { return e.db.Delete(coll, id) }

func (e *embeddedBackend) Backup(w io.Writer) error { return e.db.Backup(w) }

func (e *embeddedBackend) Verify() error { return e.db.Verify() }

func (e *embeddedBackend) QueryPage(coll, fld string, op query.Op, value string, pageSize int, pageToken string) ([]docRow, string, error) {
	filter := query.Cmp{Field: fld, Op: op, Value: value}
	if pageSize == 0 {
		rs, err := query.Execute(e.db, coll, filter)
		if err != nil {
			return nil, "", err
		}
		out := make([]docRow, 0, len(rs))
		for _, r := range rs {
			out = append(out, docRow{ID: r.ID, JSON: string(r.JSON)})
		}
		return out, "", nil
	}
	page, err := query.ExecutePage(e.db, coll, filter, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}
	out := make([]docRow, 0, len(page.Results))
	for _, r := range page.Results {
		out = append(out, docRow{ID: r.ID, JSON: string(r.JSON)})
	}
	return out, page.NextPageToken, nil
}

func (e *embeddedBackend) CreateKey(name, role string, expiresAtUnix int64) (string, string, error) {
	r, err := auth.ParseRole(role)
	if err != nil {
		return "", "", err
	}
	var expiresAt time.Time
	if expiresAtUnix != 0 {
		expiresAt = time.Unix(expiresAtUnix, 0).UTC()
	}
	token, keyID, err := auth.CreateKeyWithExpiry(e.db.Store(), name, r, expiresAt)
	return keyID, token, err
}

func (e *embeddedBackend) ListKeys() ([]keyRow, error) {
	recs, err := auth.List(e.db.Store())
	if err != nil {
		return nil, err
	}
	out := make([]keyRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, keyRow{KeyID: r.KeyID, Name: r.Name, Role: r.Role.String(), CreatedUnix: r.CreatedUnix, ExpiresUnix: r.ExpiresUnix})
	}
	return out, nil
}

func (e *embeddedBackend) RevokeKey(keyID string) error {
	return auth.Revoke(e.db.Store(), keyID)
}

// --- remote backend (gRPC client to a KoraDB-server) ---

type remoteBackend struct {
	conn   *grpc.ClientConn
	client pb.KoraDBClient
	token  string
}

func openRemote(addr, token string, tlsCfg *tls.Config) (*remoteBackend, error) {
	creds := insecure.NewCredentials()
	if tlsCfg != nil {
		creds = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &remoteBackend{conn: conn, client: pb.NewKoraDBClient(conn), token: token}, nil
}

func (r *remoteBackend) Close() error { return r.conn.Close() }

func (r *remoteBackend) ctx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	if r.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+r.token)
	}
	return ctx, cancel
}

func (r *remoteBackend) PutSchema(name, source string) (int, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.PutSchema(ctx, &pb.PutSchemaRequest{Name: name, ProtoSource: source})
	if err != nil {
		return 0, err
	}
	return int(resp.GetVersion()), nil
}

func (r *remoteBackend) ListSchemas() ([]schemaRow, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.ListSchemas(ctx, &pb.ListSchemasRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]schemaRow, 0, len(resp.GetSchemas()))
	for _, s := range resp.GetSchemas() {
		out = append(out, schemaRow{Name: s.GetName(), Version: int(s.GetVersion())})
	}
	return out, nil
}

func (r *remoteBackend) CreateCollection(name, msgType, keyField string, indexes []string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.client.CreateCollection(ctx, &pb.CreateCollectionRequest{
		Name: name, MessageType: msgType, KeyField: keyField, Indexes: indexes,
	})
	return err
}

func (r *remoteBackend) ListCollections() ([]collRow, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.ListCollections(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]collRow, 0, len(resp.GetCollections()))
	for _, c := range resp.GetCollections() {
		out = append(out, collRow{Name: c.GetName(), MessageType: c.GetMessageType(), KeyKind: c.GetKeyKind(), Indexes: c.GetIndexes()})
	}
	return out, nil
}

func (r *remoteBackend) Insert(coll, json string) (string, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.Insert(ctx, &pb.InsertRequest{Collection: coll, Json: json})
	if err != nil {
		return "", err
	}
	return resp.GetId(), nil
}

func (r *remoteBackend) Get(coll, id string) (string, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.Get(ctx, &pb.GetRequest{Collection: coll, Id: id})
	if err != nil {
		return "", err
	}
	return resp.GetJson(), nil
}

func (r *remoteBackend) Update(coll, id, json string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.client.Update(ctx, &pb.UpdateRequest{Collection: coll, Id: id, Json: json})
	return err
}

func (r *remoteBackend) Delete(coll, id string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.client.Delete(ctx, &pb.DeleteRequest{Collection: coll, Id: id})
	return err
}

func (r *remoteBackend) Backup(io.Writer) error {
	return fmt.Errorf("backup is available only for an embedded database; remote backup operations are not implemented")
}

func (r *remoteBackend) Verify() error {
	return fmt.Errorf("verify is available only for an embedded database; remote verification operations are not implemented")
}

func (r *remoteBackend) QueryPage(coll, fld string, op query.Op, value string, pageSize int, pageToken string) ([]docRow, string, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.Query(ctx, &pb.QueryRequest{
		Collection: coll,
		Filter:     &pb.Filter{Node: &pb.Filter_Cmp{Cmp: &pb.Cmp{Field: fld, Op: toWireOp(op), Value: value}}},
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
	})
	if err != nil {
		return nil, "", err
	}
	out := make([]docRow, 0, len(resp.GetResults()))
	for _, d := range resp.GetResults() {
		out = append(out, docRow{ID: d.GetId(), JSON: d.GetJson()})
	}
	return out, resp.GetNextPageToken(), nil
}

func (r *remoteBackend) CreateKey(name, role string, expiresAtUnix int64) (string, string, error) {
	wireRole, err := roleToWire(role)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.CreateKey(ctx, &pb.CreateKeyRequest{Name: name, Role: wireRole, ExpiresAtUnix: expiresAtUnix})
	if err != nil {
		return "", "", err
	}
	return resp.GetKeyId(), resp.GetToken(), nil
}

func (r *remoteBackend) ListKeys() ([]keyRow, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	resp, err := r.client.ListKeys(ctx, &pb.ListKeysRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]keyRow, 0, len(resp.GetKeys()))
	for _, k := range resp.GetKeys() {
		out = append(out, keyRow{KeyID: k.GetKeyId(), Name: k.GetName(), Role: roleFromWire(k.GetRole()), CreatedUnix: k.GetCreatedAtUnix(), ExpiresUnix: k.GetExpiresAtUnix()})
	}
	return out, nil
}

func (r *remoteBackend) RevokeKey(keyID string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.client.RevokeKey(ctx, &pb.RevokeKeyRequest{KeyId: keyID})
	return err
}

func roleToWire(role string) (pb.Role, error) {
	switch role {
	case "readonly", "ro":
		return pb.Role_ROLE_READONLY, nil
	case "readwrite", "rw":
		return pb.Role_ROLE_READWRITE, nil
	case "admin":
		return pb.Role_ROLE_ADMIN, nil
	default:
		return pb.Role_ROLE_UNSPECIFIED, fmt.Errorf("unknown role %q (want readonly|readwrite|admin)", role)
	}
}

func roleFromWire(r pb.Role) string {
	switch r {
	case pb.Role_ROLE_READONLY:
		return "readonly"
	case pb.Role_ROLE_READWRITE:
		return "readwrite"
	case pb.Role_ROLE_ADMIN:
		return "admin"
	default:
		return "none"
	}
}

func toWireOp(op query.Op) pb.Op {
	switch op {
	case query.Eq:
		return pb.Op_OP_EQ
	case query.Ne:
		return pb.Op_OP_NE
	case query.Gt:
		return pb.Op_OP_GT
	case query.Gte:
		return pb.Op_OP_GTE
	case query.Lt:
		return pb.Op_OP_LT
	case query.Lte:
		return pb.Op_OP_LTE
	default:
		return pb.Op_OP_UNSPECIFIED
	}
}

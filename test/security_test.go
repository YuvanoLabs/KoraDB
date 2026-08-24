package dbtest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "KoraDB/api/gen/KoraDBv1"
	"KoraDB/internal/auth"
	"KoraDB/internal/certgen"
	"KoraDB/internal/engine"
	"KoraDB/internal/server"
)

// secured starts a TLS + auth gRPC server on loopback with ephemeral in-memory
// certs. It returns the address, the CA PEM (for the client to trust), and the
// engine.DB so the test can mint keys directly.
func secured(t *testing.T, mtls bool) (addr string, caPEM []byte, db *engine.DB) {
	t.Helper()
	db, err := engine.Open(filepath.Join(t.TempDir(), "sec.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	bundle, err := certgen.Generate([]string{"127.0.0.1"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(bundle.ServerCertPEM, bundle.ServerKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if mtls {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(bundle.CACertPEM)
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(cfg)),
		grpc.ChainUnaryInterceptor(server.AuditInterceptor(), server.AuthInterceptor(db.Store())),
	)
	pb.RegisterKoraDBServer(srv, server.New(db))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	return lis.Addr().String(), bundle.CACertPEM, db
}

func tlsClient(t *testing.T, addr string, caPEM []byte) pb.KoraDBClient {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewKoraDBClient(conn)
}

func withToken(token string) context.Context {
	ctx := context.Background()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	return ctx
}

// TestAuthDenials is the core proof: every unauthorized path is rejected with
// the right code, and authorized paths succeed.
func TestAuthDenials(t *testing.T) {
	addr, ca, db := secured(t, false)
	client := tlsClient(t, addr, ca)

	adminTok, _, _ := auth.CreateKey(db.Store(), "admin", auth.RoleAdmin)
	roTok, _, _ := auth.CreateKey(db.Store(), "reader", auth.RoleReadOnly)

	// Admin sets up schema + collection (privileged) ? should succeed.
	if _, err := client.PutSchema(withToken(adminTok), &pb.PutSchemaRequest{Name: "user.proto", ProtoSource: userProto}); err != nil {
		t.Fatalf("admin PutSchema: %v", err)
	}
	if _, err := client.CreateCollection(withToken(adminTok), &pb.CreateCollectionRequest{
		Name: "users", MessageType: "example.User", KeyField: "email",
	}); err != nil {
		t.Fatalf("admin CreateCollection: %v", err)
	}

	// 1. No token -> Unauthenticated.
	_, err := client.Get(withToken(""), &pb.GetRequest{Collection: "users", Id: "x"})
	assertCode(t, "no token", err, codes.Unauthenticated)

	// 2. Garbage token -> Unauthenticated.
	_, err = client.Get(withToken("kdb_deadbeefdeadbeef_"+repeat64()), &pb.GetRequest{Collection: "users", Id: "x"})
	assertCode(t, "bad token", err, codes.Unauthenticated)

	// 3. Readonly principal calling Insert (a write) -> PermissionDenied.
	_, err = client.Insert(withToken(roTok), &pb.InsertRequest{Collection: "users", Json: `{"email":"a@x.com"}`})
	assertCode(t, "readonly insert", err, codes.PermissionDenied)

	// 4. Readonly calling PutSchema (admin) -> PermissionDenied.
	_, err = client.PutSchema(withToken(roTok), &pb.PutSchemaRequest{Name: "x.proto", ProtoSource: userProto})
	assertCode(t, "readonly putschema", err, codes.PermissionDenied)

	// 5. Readonly CAN read (Query) -> OK.
	if _, err := client.Query(withToken(roTok), &pb.QueryRequest{Collection: "users"}); err != nil {
		t.Fatalf("readonly query should be allowed: %v", err)
	}

	// 6. Admin can write.
	if _, err := client.Insert(withToken(adminTok), &pb.InsertRequest{Collection: "users", Json: `{"email":"a@x.com"}`}); err != nil {
		t.Fatalf("admin insert: %v", err)
	}

	// 7. Revocation takes effect immediately (no restart).
	tempTok, tempID, _ := auth.CreateKey(db.Store(), "temp", auth.RoleReadOnly)
	if _, err := client.Query(withToken(tempTok), &pb.QueryRequest{Collection: "users"}); err != nil {
		t.Fatalf("temp key should work before revoke: %v", err)
	}
	if err := auth.Revoke(db.Store(), tempID); err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(withToken(tempTok), &pb.QueryRequest{Collection: "users"})
	assertCode(t, "revoked key", err, codes.Unauthenticated)
}

// TestPlaintextClientRejectedByTLSServer proves tokens can't be sent in the
// clear: a non-TLS client cannot talk to the TLS server.
func TestPlaintextClientRejectedByTLSServer(t *testing.T) {
	addr, _, db := secured(t, false)
	adminTok, _, _ := auth.CreateKey(db.Store(), "admin", auth.RoleAdmin)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewKoraDBClient(conn)

	ctx, cancel := context.WithTimeout(withToken(adminTok), 3*time.Second)
	defer cancel()
	if _, err := client.ListSchemas(ctx, &pb.ListSchemasRequest{}); err == nil {
		t.Fatal("plaintext client must NOT succeed against a TLS server")
	}
}

// TestMTLSRequiresClientCert proves an mTLS server rejects a client that has no
// client certificate, even though it trusts the server's CA.
func TestMTLSRequiresClientCert(t *testing.T) {
	addr, ca, db := secured(t, true) // mTLS required
	_, _, _ = auth.CreateKey(db.Store(), "admin", auth.RoleAdmin)

	// Client trusts the CA but presents NO client cert.
	client := tlsClient(t, addr, ca)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ListSchemas(ctx, &pb.ListSchemasRequest{}); err == nil {
		t.Fatal("mTLS server must reject a client with no client certificate")
	}
}

func assertCode(t *testing.T, what string, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("%s: got code %s (err=%v), want %s", what, status.Code(err), err, want)
	}
}

func repeat64() string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}


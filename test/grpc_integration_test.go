package dbtest

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/YuvanoLabs/KoraDB/api/gen/KoraDBv1"
	"github.com/YuvanoLabs/KoraDB/internal/engine"
	"github.com/YuvanoLabs/KoraDB/internal/server"
)

const userProto = `
syntax = "proto3";
package example;
message User {
  string name = 1;
  string email = 2;
  string city = 3;
}`

// TestGRPCEndToEnd starts a real gRPC server on a loopback port and drives the
// whole stack through the generated client: schema -> collection -> insert ->
// get -> indexed query -> schema evolution -> delete. This proves the network
// path, not just the embedded engine.
func TestGRPCEndToEnd(t *testing.T) {
	db, err := engine.Open(filepath.Join(t.TempDir(), "grpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterKoraDBServer(srv, server.New(db))
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewKoraDBClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Register schema.
	pr, err := client.PutSchema(ctx, &pb.PutSchemaRequest{Name: "user.proto", ProtoSource: userProto})
	if err != nil {
		t.Fatalf("PutSchema: %v", err)
	}
	if pr.GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", pr.GetVersion())
	}

	// 2. Create collection: key=email, index on city.
	if _, err := client.CreateCollection(ctx, &pb.CreateCollectionRequest{
		Name: "users", MessageType: "example.User", KeyField: "email", Indexes: []string{"city"},
	}); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// 3. Insert.
	for _, doc := range []string{
		`{"name":"Alice","email":"alice@x.com","city":"NYC"}`,
		`{"name":"Bob","email":"bob@x.com","city":"LA"}`,
		`{"name":"Carol","email":"carol@x.com","city":"NYC"}`,
	} {
		if _, err := client.Insert(ctx, &pb.InsertRequest{Collection: "users", Json: doc}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// 4. Get by key.
	gr, err := client.Get(ctx, &pb.GetRequest{Collection: "users", Id: "alice@x.com"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gr.GetJson() == "" {
		t.Fatal("Get returned empty json")
	}

	// 5. Query by indexed field city == NYC -> 2 docs.
	qr, err := client.Query(ctx, &pb.QueryRequest{
		Collection: "users",
		Filter:     &pb.Filter{Node: &pb.Filter_Cmp{Cmp: &pb.Cmp{Field: "city", Op: pb.Op_OP_EQ, Value: "NYC"}}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(qr.GetResults()) != 2 {
		t.Fatalf("query city==NYC returned %d, want 2", len(qr.GetResults()))
	}

	// 5b. The same query can opt into an opaque continuation-token page.
	firstPage, err := client.Query(ctx, &pb.QueryRequest{
		Collection: "users",
		Filter:     &pb.Filter{Node: &pb.Filter_Cmp{Cmp: &pb.Cmp{Field: "city", Op: pb.Op_OP_EQ, Value: "NYC"}}},
		PageSize:   1,
	})
	if err != nil {
		t.Fatalf("first paged query: %v", err)
	}
	if len(firstPage.GetResults()) != 1 || firstPage.GetNextPageToken() == "" {
		t.Fatalf("first paged query = %#v, want one result and continuation token", firstPage)
	}
	secondPage, err := client.Query(ctx, &pb.QueryRequest{
		Collection: "users",
		Filter:     &pb.Filter{Node: &pb.Filter_Cmp{Cmp: &pb.Cmp{Field: "city", Op: pb.Op_OP_EQ, Value: "NYC"}}},
		PageSize:   1,
		PageToken:  firstPage.GetNextPageToken(),
	})
	if err != nil {
		t.Fatalf("second paged query: %v", err)
	}
	if len(secondPage.GetResults()) != 1 || secondPage.GetNextPageToken() != "" {
		t.Fatalf("second paged query = %#v, want one final result", secondPage)
	}
	if firstPage.GetResults()[0].GetId() == secondPage.GetResults()[0].GetId() {
		t.Fatal("paged query returned a duplicate document")
	}

	// 6. Schema evolution over the wire: add a field, old docs still readable.
	evolved := `
syntax = "proto3";
package example;
message User {
  string name = 1;
  string email = 2;
  string city = 3;
  int32 age = 4;
}`
	pr2, err := client.PutSchema(ctx, &pb.PutSchemaRequest{Name: "user.proto", ProtoSource: evolved})
	if err != nil {
		t.Fatalf("evolve PutSchema: %v", err)
	}
	if pr2.GetVersion() != 2 {
		t.Fatalf("evolved version = %d, want 2", pr2.GetVersion())
	}
	if _, err := client.Get(ctx, &pb.GetRequest{Collection: "users", Id: "alice@x.com"}); err != nil {
		t.Fatalf("old doc unreadable after evolution: %v", err)
	}

	// 7. Delete.
	if _, err := client.Delete(ctx, &pb.DeleteRequest{Collection: "users", Id: "bob@x.com"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.Get(ctx, &pb.GetRequest{Collection: "users", Id: "bob@x.com"}); err == nil {
		t.Fatal("expected NotFound after delete")
	}

	t.Log("gRPC end-to-end verified: schema, CRUD, indexed query, evolution, delete over the network")
}

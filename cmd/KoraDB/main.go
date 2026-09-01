// Command KoraDB is the KoraDB CLI. It works two ways from the same binary:
//
//   - Embedded (default): operates directly on a local .db file.
//   - Remote: with --server HOST:PORT, sends every command to a KoraDB-server
//     over gRPC.
//
// Usage:
//
//	KoraDB --db users.db schema add user.proto ./examples/user.proto
//	KoraDB --server 127.0.0.1:50051 query users city == NYC
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/YuvanoLabs/KoraDB/internal/buildinfo"
	"github.com/YuvanoLabs/KoraDB/internal/query"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	dbPath := "KoraDB.db"
	serverAddr := ""
	token := os.Getenv("KoraDB_TOKEN")
	tlsCA := ""
	tlsServerName := ""
	tlsSkipVerify := false
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", a)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case strings.HasPrefix(a, "--db="):
			dbPath = strings.TrimPrefix(a, "--db=")
		case a == "--db":
			if dbPath, err = next(); err != nil {
				return err
			}
		case strings.HasPrefix(a, "--server="):
			serverAddr = strings.TrimPrefix(a, "--server=")
		case a == "--server":
			if serverAddr, err = next(); err != nil {
				return err
			}
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		case a == "--token":
			if token, err = next(); err != nil {
				return err
			}
		case strings.HasPrefix(a, "--tls-ca="):
			tlsCA = strings.TrimPrefix(a, "--tls-ca=")
		case a == "--tls-ca":
			if tlsCA, err = next(); err != nil {
				return err
			}
		case strings.HasPrefix(a, "--tls-server-name="):
			tlsServerName = strings.TrimPrefix(a, "--tls-server-name=")
		case a == "--tls-server-name":
			if tlsServerName, err = next(); err != nil {
				return err
			}
		case a == "--tls-skip-verify":
			tlsSkipVerify = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		usage()
		return nil
	}
	if rest[0] == "version" {
		if len(rest) != 1 {
			return fmt.Errorf("usage: version")
		}
		fmt.Println(buildinfo.String())
		return nil
	}

	tlsCfg, err := clientTLS(tlsCA, tlsServerName, tlsSkipVerify)
	if err != nil {
		return err
	}
	be, err := openBackend(dbPath, serverAddr, token, tlsCfg)
	if err != nil {
		return err
	}
	defer be.Close()

	switch rest[0] {
	case "schema":
		return cmdSchema(be, rest[1:])
	case "collection":
		return cmdCollection(be, rest[1:])
	case "insert":
		return cmdInsert(be, rest[1:])
	case "get":
		return cmdGet(be, rest[1:])
	case "update":
		return cmdUpdate(be, rest[1:])
	case "delete":
		return cmdDelete(be, rest[1:])
	case "backup":
		return cmdBackup(be, rest[1:])
	case "verify":
		return cmdVerify(be, rest[1:])
	case "query":
		return cmdQuery(be, rest[1:])
	case "key":
		return cmdKey(be, rest[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try `KoraDB help`)", rest[0])
	}
}

// openBackend selects the remote gRPC backend when --server is set, otherwise
// the embedded file backend.
func openBackend(dbPath, serverAddr, token string, tlsCfg *tls.Config) (backend, error) {
	if serverAddr != "" {
		if token != "" && tlsCfg == nil {
			return nil, fmt.Errorf("refusing to send an API token without TLS: provide --tls-ca, --tls-server-name, or --tls-skip-verify for development")
		}
		return openRemote(serverAddr, token, tlsCfg)
	}
	return openEmbedded(dbPath)
}

// clientTLS builds the client TLS config from flags. It returns nil (plaintext)
// only for unauthenticated development connections.
func clientTLS(caFile, serverName string, skipVerify bool) (*tls.Config, error) {
	if caFile == "" && serverName == "" && !skipVerify {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName, InsecureSkipVerify: skipVerify}
	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read --tls-ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("--tls-ca %q has no certificates", caFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

func cmdSchema(be backend, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("schema: expected `add` or `list`")
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: schema add <name> <file.proto>")
		}
		name, path := args[1], args[2]
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		v, err := be.PutSchema(name, string(src))
		if err != nil {
			return err
		}
		fmt.Printf("registered schema %q (version %d)\n", name, v)
		return nil
	case "list":
		schemas, err := be.ListSchemas()
		if err != nil {
			return err
		}
		for _, s := range schemas {
			fmt.Printf("%s\tv%d\n", s.Name, s.Version)
		}
		return nil
	default:
		return fmt.Errorf("schema: unknown subcommand %q", args[0])
	}
}

func cmdCollection(be backend, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("collection: expected `create` or `list`")
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			return fmt.Errorf("usage: collection create <name> <messageType> [--key-field=F] [--index=F1,F2]")
		}
		name, msgType := args[1], args[2]
		keyField := ""
		var indexes []string
		for _, a := range args[3:] {
			switch {
			case strings.HasPrefix(a, "--key-field="):
				keyField = strings.TrimPrefix(a, "--key-field=")
			case strings.HasPrefix(a, "--index="):
				indexes = strings.Split(strings.TrimPrefix(a, "--index="), ",")
			default:
				return fmt.Errorf("collection create: unknown flag %q", a)
			}
		}
		if err := be.CreateCollection(name, msgType, keyField, indexes); err != nil {
			return err
		}
		fmt.Printf("created collection %q (type %s)\n", name, msgType)
		return nil
	case "list":
		colls, err := be.ListCollections()
		if err != nil {
			return err
		}
		for _, c := range colls {
			idx := ""
			if len(c.Indexes) > 0 {
				idx = " indexes=" + strings.Join(c.Indexes, ",")
			}
			fmt.Printf("%s\ttype=%s key=%s%s\n", c.Name, c.MessageType, c.KeyKind, idx)
		}
		return nil
	default:
		return fmt.Errorf("collection: unknown subcommand %q", args[0])
	}
}

func cmdInsert(be backend, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: insert <collection> <json>  (use - to read JSON from stdin)")
	}
	coll, doc := args[0], args[1]
	if doc == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		doc = string(b)
	}
	id, err := be.Insert(coll, doc)
	if err != nil {
		return err
	}
	fmt.Printf("inserted id=%s\n", id)
	return nil
}

func cmdGet(be backend, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: get <collection> <id>")
	}
	j, err := be.Get(args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Println(j)
	return nil
}

func cmdUpdate(be backend, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: update <collection> <id> <json>  (use - to read JSON from stdin)")
	}
	coll, id, doc := args[0], args[1], args[2]
	if doc == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		doc = string(b)
	}
	if err := be.Update(coll, id, doc); err != nil {
		return err
	}
	fmt.Println("updated")
	return nil
}

func cmdDelete(be backend, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: delete <collection> <id>")
	}
	if err := be.Delete(args[0], args[1]); err != nil {
		return err
	}
	fmt.Println("deleted")
	return nil
}

func cmdBackup(be backend, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: backup <output.db>")
	}
	output := args[0]
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("refusing to overwrite existing backup %q", output)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect backup destination %q: %w", output, err)
	}

	dir := filepath.Dir(output)
	tmp, err := os.CreateTemp(dir, ".KoraDB-backup-*")
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("secure backup file: %w", err)
	}
	if err := be.Backup(tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("finalize backup: %w", err)
	}
	if err := os.Rename(tmpName, output); err != nil {
		return fmt.Errorf("publish backup %q: %w", output, err)
	}
	committed = true
	fmt.Printf("backup written to %s\n", output)
	return nil
}

func cmdVerify(be backend, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: verify")
	}
	if err := be.Verify(); err != nil {
		return err
	}
	fmt.Println("storage integrity verified")
	return nil
}

func cmdQuery(be backend, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: query <collection> <field> <op> <value> [--page-size=N] [--page-token=T]   (op: == != > >= < <=)")
	}
	coll, fld, opStr, val := args[0], args[1], args[2], args[3]
	op, err := parseOp(opStr)
	if err != nil {
		return err
	}
	pageSize := 0
	pageToken := ""
	for _, arg := range args[4:] {
		switch {
		case strings.HasPrefix(arg, "--page-size="):
			pageSize, err = strconv.Atoi(strings.TrimPrefix(arg, "--page-size="))
			if err != nil || pageSize <= 0 {
				return fmt.Errorf("query: --page-size must be a positive integer")
			}
		case strings.HasPrefix(arg, "--page-token="):
			pageToken = strings.TrimPrefix(arg, "--page-token=")
			if pageToken == "" {
				return fmt.Errorf("query: --page-token must not be empty")
			}
		default:
			return fmt.Errorf("query: unknown flag %q", arg)
		}
	}
	if pageToken != "" && pageSize == 0 {
		return fmt.Errorf("query: --page-token requires --page-size")
	}
	results, nextPageToken, err := be.QueryPage(coll, fld, op, val, pageSize, pageToken)
	if err != nil {
		return err
	}
	fmt.Printf("%d result(s):\n", len(results))
	for _, r := range results {
		fmt.Printf("  [%s] %s\n", r.ID, r.JSON)
	}
	if nextPageToken != "" {
		fmt.Printf("next_page_token=%s\n", nextPageToken)
	}
	return nil
}

func cmdKey(be backend, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("key: expected `create`, `list`, or `revoke`")
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			return fmt.Errorf("usage: key create <name> <role> [--expires-at=RFC3339]   (role: readonly|readwrite|admin)")
		}
		var expiresAtUnix int64
		for _, arg := range args[3:] {
			if !strings.HasPrefix(arg, "--expires-at=") {
				return fmt.Errorf("key create: unknown flag %q", arg)
			}
			expiresAt, err := time.Parse(time.RFC3339, strings.TrimPrefix(arg, "--expires-at="))
			if err != nil {
				return fmt.Errorf("key create: --expires-at must be RFC3339: %w", err)
			}
			expiresAtUnix = expiresAt.UTC().Unix()
		}
		keyID, token, err := be.CreateKey(args[1], args[2], expiresAtUnix)
		if err != nil {
			return err
		}
		fmt.Printf("created key id=%s\n\n  %s\n\nShown once — store it securely.\n", keyID, token)
		return nil
	case "list":
		keys, err := be.ListKeys()
		if err != nil {
			return err
		}
		for _, k := range keys {
			expires := "never"
			if k.ExpiresUnix != 0 {
				expires = time.Unix(k.ExpiresUnix, 0).UTC().Format(time.RFC3339)
			}
			fmt.Printf("%s\t%s\texpires=%s\t%s\n", k.KeyID, k.Role, expires, k.Name)
		}
		return nil
	case "revoke":
		if len(args) != 2 {
			return fmt.Errorf("usage: key revoke <key-id>")
		}
		if err := be.RevokeKey(args[1]); err != nil {
			return err
		}
		fmt.Println("revoked")
		return nil
	default:
		return fmt.Errorf("key: unknown subcommand %q", args[0])
	}
}

func parseOp(s string) (query.Op, error) {
	switch s {
	case "==", "eq":
		return query.Eq, nil
	case "!=", "ne":
		return query.Ne, nil
	case ">", "gt":
		return query.Gt, nil
	case ">=", "gte":
		return query.Gte, nil
	case "<", "lt":
		return query.Lt, nil
	case "<=", "lte":
		return query.Lte, nil
	default:
		return 0, fmt.Errorf("unknown operator %q", s)
	}
}

func usage() {
	fmt.Print(`KoraDB — a protobuf-native file-based database

Usage:
  KoraDB [--db <file> | --server <host:port>] [auth/tls flags] <command> [args]

By default operates on a local database file (--db, default KoraDB.db).
With --server, sends commands to a KoraDB-server over gRPC instead.

Remote flags:
  --token <token>          API token (or set KoraDB_TOKEN; requires TLS)
  --tls-ca <file>          trust this CA for the server's TLS cert
  --tls-server-name <name> override the TLS name to verify
  --tls-skip-verify        DANGER: skip TLS verification (dev only)

Commands:
  schema add <name> <file.proto>          register or evolve a schema
  schema list                             list registered schemas
  collection create <name> <type> [..]    create a collection bound to a message type
        flags: --key-field=F  --index=F1,F2
  collection list                         list collections
  insert <collection> <json>              insert a document (- reads stdin)
  get <collection> <id>                   fetch a document by id
  update <collection> <id> <json>         replace a document (- reads stdin)
  delete <collection> <id>                delete a document
  backup <output.db>                       write a consistent embedded snapshot (no overwrite)
  verify                                   check embedded storage integrity
  version                                  print build identity
  query <collection> <field> <op> <value> [--page-size=N] [--page-token=T]
                                            query (op: == != > >= < <=)
  key create <name> <role> [--expires-at=RFC3339]
                                            create an API key (admin); role: readonly|readwrite|admin
  key list                                list API keys (admin)
  key revoke <key-id>                     revoke an API key (admin)
`)
}

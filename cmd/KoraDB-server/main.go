// Command KoraDB-server runs KoraDB as a gRPC service — the mongod-equivalent
// daemon. It is a single static binary with every dependency compiled in.
//
// Subcommands:
//
//	serve      run the server (default)
//	bootstrap  create the first admin API key on a db file (server must be stopped)
//	gencert    generate development TLS certificates
//
// Security is fail-closed: `serve` refuses to start without TLS + at least one
// API key, unless you pass --insecure (which disables TLS and auth and prints a
// loud warning — for local development only).
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "KoraDB/api/gen/KoraDBv1"
	"KoraDB/internal/auth"
	"KoraDB/internal/certgen"
	"KoraDB/internal/engine"
	"KoraDB/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	args := os.Args[1:]
	sub := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}

	var err error
	switch sub {
	case "serve":
		err = runServe(args)
	case "bootstrap":
		err = runBootstrap(args)
	case "gencert":
		err = runGencert(args)
	case "help", "-h", "--help":
		usage()
	default:
		err = fmt.Errorf("unknown subcommand %q (try `KoraDB-server help`)", sub)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":50051", "gRPC listen address")
	dbPath := fs.String("db", "KoraDB.db", "path to the database file")
	tlsCert := fs.String("tls-cert", "", "server TLS certificate (PEM)")
	tlsKey := fs.String("tls-key", "", "server TLS private key (PEM)")
	clientCA := fs.String("tls-client-ca", "", "CA to verify client certs (enables mTLS)")
	insecure := fs.Bool("insecure", false, "DANGER: disable TLS and authentication (dev only)")
	fs.Parse(args)

	db, err := engine.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database %q: %w", *dbPath, err)
	}
	defer db.Close()

	var opts []grpc.ServerOption
	if *insecure {
		log.Print("############################################################")
		log.Print("# WARNING: --insecure: NO TLS and NO AUTH. Anyone who can   #")
		log.Print("# reach this port has full admin access. Localhost dev only. #")
		log.Print("############################################################")
		opts = append(opts, grpc.ChainUnaryInterceptor(server.AuditInterceptor()))
	} else {
		// Secure by default: require TLS and a bootstrapped key.
		if *tlsCert == "" || *tlsKey == "" {
			return fmt.Errorf("refusing to start without TLS: provide --tls-cert and --tls-key " +
				"(or --insecure for localhost dev). Generate dev certs with `KoraDB-server gencert`")
		}
		creds, err := serverTLS(*tlsCert, *tlsKey, *clientCA)
		if err != nil {
			return err
		}
		has, err := auth.HasAnyKey(db.Store())
		if err != nil {
			return err
		}
		if !has {
			return fmt.Errorf("refusing to start with no API keys: create the first admin key with "+
				"`KoraDB-server bootstrap --db %q` while the server is stopped", *dbPath)
		}
		opts = append(opts,
			grpc.Creds(creds),
			// Audit is outermost so it records auth failures too.
			grpc.ChainUnaryInterceptor(server.AuditInterceptor(), server.AuthInterceptor(db.Store())),
		)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *addr, err)
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterKoraDBServer(grpcServer, server.New(db))

	go gracefulShutdown(grpcServer)

	mode := "TLS+auth"
	if *insecure {
		mode = "INSECURE"
	} else if *clientCA != "" {
		mode = "mTLS+auth"
	}
	log.Printf("KoraDB-server: listening on %s [%s], database %q", *addr, mode, *dbPath)
	return grpcServer.Serve(lis)
}

func serverTLS(certFile, keyFile, clientCAFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS keypair: %w", err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if clientCAFile != "" {
		caPEM, err := os.ReadFile(clientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("client CA %q contains no certificates", clientCAFile)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(cfg), nil
}

func gracefulShutdown(s *grpc.Server) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	got := <-sig
	log.Printf("KoraDB-server: received %s, shutting down gracefully", got)
	done := make(chan struct{})
	go func() { s.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		log.Print("KoraDB-server: graceful stop timed out, forcing")
		s.Stop()
	}
}

func runBootstrap(args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	dbPath := fs.String("db", "KoraDB.db", "path to the database file (server must be stopped)")
	name := fs.String("name", "admin", "principal name for the key (used in audit logs)")
	roleStr := fs.String("role", "admin", "role: readonly | readwrite | admin")
	fs.Parse(args)

	role, err := auth.ParseRole(*roleStr)
	if err != nil {
		return err
	}
	db, err := engine.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database %q (is the server still running? it holds an exclusive lock): %w", *dbPath, err)
	}
	defer db.Close()

	token, keyID, err := auth.CreateKey(db.Store(), *name, role)
	if err != nil {
		return err
	}
	fmt.Printf("Created %s key %q (id %s)\n\n", role, *name, keyID)
	fmt.Printf("  %s\n\n", token)
	fmt.Println("This token is shown ONCE and cannot be recovered. Store it securely.")
	fmt.Println("Use it with:  KoraDB --server <host:port> --token <token> ...")
	return nil
}

func runGencert(args []string) error {
	fs := flag.NewFlagSet("gencert", flag.ExitOnError)
	dir := fs.String("dir", "certs", "output directory")
	hosts := fs.String("host", "localhost,127.0.0.1", "comma-separated DNS names / IPs")
	days := fs.Int("days", 365, "validity in days")
	fs.Parse(args)

	bundle, err := certgen.Generate(strings.Split(*hosts, ","), time.Duration(*days)*24*time.Hour)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		"ca.crt":     bundle.CACertPEM,
		"server.crt": bundle.ServerCertPEM,
		"server.key": bundle.ServerKeyPEM,
	}
	for name, data := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".key") {
			mode = 0o600
		}
		if err := os.WriteFile(filepath.Join(*dir, name), data, mode); err != nil {
			return err
		}
	}
	fmt.Printf("Wrote ca.crt, server.crt, server.key to %s/\n", *dir)
	fmt.Printf("Server:  KoraDB-server serve --tls-cert %s/server.crt --tls-key %s/server.key\n", *dir, *dir)
	fmt.Printf("Client:  KoraDB --server <host:port> --tls-ca %s/ca.crt --token <token> ...\n", *dir)
	return nil
}

func usage() {
	fmt.Print(`KoraDB-server — KoraDB gRPC daemon

Usage:
  KoraDB-server <subcommand> [flags]

Subcommands:
  serve       run the server (default)
                --addr :50051  --db KoraDB.db
                --tls-cert FILE --tls-key FILE [--tls-client-ca FILE]
                --insecure   (DANGER: no TLS, no auth; dev only)
  bootstrap   create the first admin API key (run with the server stopped)
                --db KoraDB.db --name admin --role admin
  gencert     generate development TLS certificates
                --dir certs --host localhost,127.0.0.1 --days 365

Security: serve is fail-closed — it will not start without TLS and at least one
API key unless --insecure is given.
`)
}

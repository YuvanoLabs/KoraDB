// KoraDB-restore performs a safe, offline restore of a KoraDB snapshot.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"KoraDB/internal/recovery"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "KoraDB-restore:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("KoraDB-restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	maxBytes := flags.Int64("max-bytes", 0, "maximum accepted snapshot size in bytes (required)")
	overwrite := flags.Bool("overwrite", false, "replace an existing destination only after moving it to --rollback")
	rollback := flags.String("rollback", "", "new rollback database path in the destination directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("usage: KoraDB-restore --max-bytes <bytes> [--overwrite --rollback <path>] <snapshot.db> <destination.db>")
	}

	bytesWritten, err := recovery.RestoreFile(flags.Arg(0), flags.Arg(1), recovery.Options{
		MaxBytes:     *maxBytes,
		Overwrite:    *overwrite,
		RollbackPath: *rollback,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "restored %d bytes to %s\n", bytesWritten, flags.Arg(1))
	return err
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pranavb1234/neonCDC/worker/internal/metadata"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "metadata:", err)
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("use init, migrate, or schema")
	}
	store, err := metadata.Open(context.Background(), metadata.Config{Path: os.Getenv("METADATA_DB_PATH"), EncryptionKey: os.Getenv("APP_ENCRYPTION_KEY")})
	if err != nil {
		return err
	}
	defer store.Close()

	switch args[0] {
	case "migrate":
		if err := store.Migrate(context.Background()); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "Metadata migrations are up to date.")
		return err
	case "init":
		flags := flag.NewFlagSet("init", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		email := flags.String("admin-email", "", "administrator email")
		passwordStdin := flags.Bool("admin-password-stdin", false, "read administrator password from standard input")
		if err := flags.Parse(args[1:]); err != nil {
			return errors.New("invalid init arguments")
		}
		if !*passwordStdin {
			return errors.New("use --admin-password-stdin so the password is not a command-line argument")
		}
		password, err := io.ReadAll(input)
		if err != nil {
			return errors.New("could not read administrator password")
		}
		password = []byte(strings.TrimRight(string(password), "\r\n"))
		if err := store.Migrate(context.Background()); err != nil {
			return err
		}
		created, err := store.BootstrapAdministrator(context.Background(), *email, password)
		if err != nil {
			return err
		}
		if created {
			_, err = fmt.Fprintln(output, "Metadata initialized and administrator created.")
		} else {
			_, err = fmt.Fprintln(output, "Metadata initialized; administrator already exists.")
		}
		return err
	case "schema":
		if err := store.Migrate(context.Background()); err != nil {
			return err
		}
		objects, err := store.Schema(context.Background())
		if err != nil {
			return err
		}
		for _, object := range objects {
			if _, err := fmt.Fprintf(output, "%s\t%s\n", object.Type, object.Name); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("use init, migrate, or schema")
	}
}

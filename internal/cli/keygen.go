package cli

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/sign"
)

func newKeygenCmd(a *app) *cobra.Command {
	var out string
	var force bool
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate an Ed25519 key for signing requests (Web Bot Auth)",
		Long: `keygen creates an Ed25519 private key for --sign and prints its key ID and
the key directory JSON to publish at /.well-known/http-message-signatures-directory.

Pass an existing key with --key to print its directory without generating one.`,
		Example: `  acurl keygen -o mybot.key
  acurl keygen --key mybot.key > http-message-signatures-directory`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			existing, _ := cmd.Flags().GetString("key")
			var key ed25519.PrivateKey
			var err error
			if existing != "" {
				key, err = sign.LoadPrivateKey(existing)
			} else {
				key, err = a.generateKey(out, force)
			}
			if err != nil {
				return err
			}
			pub := key.Public().(ed25519.PublicKey)
			dir, err := sign.Directory(pub)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stderr, "Key ID (JWK thumbprint): %s\n", sign.Thumbprint(pub))
			fmt.Fprintf(a.stderr, "Publish this at https://<your-domain>/.well-known/http-message-signatures-directory:\n")
			_, err = fmt.Fprintln(a.stdout, string(dir))
			return err
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "acurl-signing.key", "where to write the new private key")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing key file")
	cmd.Flags().String("key", "", "print the directory for an existing key instead of generating one")
	return cmd
}

func (a *app) generateKey(path string, force bool) (ed25519.PrivateKey, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return nil, fmt.Errorf("%s already exists (use --force to overwrite, or --key to reuse it)", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, err := sign.GenerateKey()
	if err != nil {
		return nil, err
	}
	pemBytes, err := sign.EncodePrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, err
	}
	fmt.Fprintf(a.stderr, "Wrote private key to %s (keep it secret)\n", path)
	return key, nil
}

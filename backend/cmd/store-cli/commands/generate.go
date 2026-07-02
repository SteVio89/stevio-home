package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
)

func GenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate secrets and keys",
	}
	cmd.AddCommand(generateSigningSecretCmd(), generateSecretCmd(), generateSaltCmd())
	return cmd
}

func generateSigningSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "signing-secret",
		Short: "Generate a random 64-char hex signing key secret",
		Long:  "Generates SIGNING_KEY_SECRET for encrypting Ed25519 private keys at rest. Signing keys themselves are managed via the admin UI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printRandomHex("SIGNING_KEY_SECRET")
		},
	}
}

func generateSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secret",
		Short: "Generate a random 64-char hex session secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printRandomHex("SESSION_SECRET")
		},
	}
}

func generateSaltCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "salt",
		Short: "Generate a random 64-char hex email hash salt",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printRandomHex("EMAIL_HASH_SALT")
		},
	}
}

func printRandomHex(label string) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate random bytes: %w", err)
	}
	fmt.Println(label + "=" + hex.EncodeToString(b))
	return nil
}

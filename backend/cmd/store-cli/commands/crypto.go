package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/SteVio89/stevio-home/crypto"
	"github.com/spf13/cobra"
)

func HashEmailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hash-email <email>",
		Short: "Compute HMAC-SHA256 hash for an email address",
		Long:  "Requires EMAIL_HASH_SALT env var. Useful for looking up hashed emails in the database.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			salt := os.Getenv("EMAIL_HASH_SALT")
			if salt == "" {
				return fmt.Errorf("EMAIL_HASH_SALT env var is not set")
			}
			email := strings.TrimSpace(args[0])
			hash := crypto.HashEmail(email, salt)
			fmt.Println(hash)
			return nil
		},
	}
}

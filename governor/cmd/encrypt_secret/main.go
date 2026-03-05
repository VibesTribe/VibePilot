package main

import (
	"fmt"
	"os"

	"github.com/vibepilot/governor/internal/vault"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: encrypt_secret <VAULT_KEY> <secret_value>")
		os.Exit(1)
	}

	vaultKey := os.Args[1]
	secretValue := os.Args[2]

	encrypted, err := vault.Encrypt(secretValue, vaultKey)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(encrypted)
}

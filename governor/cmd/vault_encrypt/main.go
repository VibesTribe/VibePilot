package main

import (
	"fmt"
	"log"
	"os"

	"github.com/vibepilot/governor/internal/vault"
)

func main() {
	masterKey := os.Getenv("VAULT_KEY")
	if masterKey == "" {
		log.Fatal("VAULT_KEY not set")
	}

	plaintext := os.Args[1]
	if plaintext == "" {
		log.Fatal("Usage: go run cmd/vault_encrypt/main.go <plaintext>")
	}

	encrypted, err := vault.Encrypt(plaintext, masterKey)
	if err != nil {
		log.Fatalf("Encrypt failed: %v", err)
	}
	fmt.Print(encrypted)
}

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

func main() {
	masterKey := os.Getenv("VAULT_KEY")
	if masterKey == "" {
		fmt.Fprintln(os.Stderr, "VAULT_KEY not set")
		os.Exit(1)
	}

	for _, encrypted := range os.Args[1:] {
		ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			fmt.Fprintf(os.Stderr, "base64 decode: %v\n", err)
			continue
		}

		if len(ciphertext) < 29 {
			fmt.Fprintln(os.Stderr, "ciphertext too short")
			continue
		}

		salt := ciphertext[:16]
		nonce := ciphertext[16:28]
		actualCt := ciphertext[28:]

		key := pbkdf2.Key([]byte(masterKey), salt, 100000, 32, sha256.New)

		block, err := aes.NewCipher(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cipher: %v\n", err)
			continue
		}

		aesgcm, err := cipher.NewGCM(block)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcm: %v\n", err)
			continue
		}

		plaintext, err := aesgcm.Open(nil, nonce, actualCt, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decrypt: %v\n", err)
			continue
		}
		fmt.Println(string(plaintext))
	}
}

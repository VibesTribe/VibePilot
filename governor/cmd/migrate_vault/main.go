// migrate_vault is a one-time migration tool for re-encrypting vault secrets.
// DEPRECATED: This tool was used during migration from Supabase to local Postgres.
// Use "governor vault" CLI for vault operations instead.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Iterations = 100000
	saltSize         = 16
	nonceSize        = 12
	keySize          = 32
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: migrate_vault <SUPABASE_URL> <SERVICE_KEY> <VAULT_KEY>")
		os.Exit(1)
	}

	supabaseURL := os.Args[1]
	serviceKey := os.Args[2]
	vaultKey := os.Args[3]

	// Fetch all secrets
	secrets, err := fetchSecrets(supabaseURL, serviceKey)
	if err != nil {
		log.Fatalf("Failed to fetch secrets: %v", err)
	}

	fmt.Printf("Found %d secrets to migrate\n", len(secrets))

	// For each secret, decrypt with OLD logic, encrypt with NEW logic
	for _, secret := range secrets {
		fmt.Printf("Migrating %s...\n", secret.KeyName)

		// Decrypt with OLD logic (double derivation)
		plaintext, err := decryptOld(secret.EncryptedValue, vaultKey)
		if err != nil {
			log.Printf("Failed to decrypt %s: %v", secret.KeyName, err)
			continue
		}

		// Encrypt with NEW logic (single derivation)
		newEncrypted, err := encryptNew(plaintext, vaultKey)
		if err != nil {
			log.Printf("Failed to encrypt %s: %v", secret.KeyName, err)
			continue
		}

		// Update in database
		if err := updateSecret(supabaseURL, serviceKey, secret.KeyName, newEncrypted); err != nil {
			log.Printf("Failed to update %s: %v", secret.KeyName, err)
			continue
		}

		fmt.Printf("  ✓ %s migrated\n", secret.KeyName)
	}

	fmt.Println("Migration complete!")
}

type Secret struct {
	KeyName        string `json:"key_name"`
	EncryptedValue string `json:"encrypted_value"`
}

func fetchSecrets(baseURL, serviceKey string) ([]Secret, error) {
	url := baseURL + "/rest/v1/secrets_vault?select=key_name,encrypted_value"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Authorization", "Bearer "+serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var secrets []Secret
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return nil, err
	}

	return secrets, nil
}

func updateSecret(baseURL, serviceKey, keyName, encryptedValue string) error {
	url := fmt.Sprintf("%s/rest/v1/secrets_vault?key_name=eq.%s", baseURL, keyName)

	payload := map[string]string{
		"encrypted_value": encryptedValue,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(body))
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("update failed with status %d", resp.StatusCode)
	}

	return nil
}

// OLD decryption logic (double derivation)
func decryptOld(encrypted, masterKey string) (string, error) {
	// First derivation with fixed salt
	fixedSalt := sha256.Sum256([]byte("vibepilot-vault-portable-salt-v1"))
	key1 := pbkdf2.Key([]byte(masterKey), fixedSalt[:saltSize], pbkdf2Iterations, keySize, sha256.New)

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(ciphertext) < saltSize+nonceSize+1 {
		return "", fmt.Errorf("ciphertext too short")
	}

	salt := ciphertext[:saltSize]
	nonce := ciphertext[saltSize : saltSize+nonceSize]
	actualCiphertext := ciphertext[saltSize+nonceSize:]

	// Second derivation with ciphertext salt
	key2 := pbkdf2.Key(key1, salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key2)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// NEW encryption logic (single derivation)
func encryptNew(plaintext, masterKey string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	key := pbkdf2.Key([]byte(masterKey), salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	result := append(salt, nonce...)
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Package vault provides encrypted secret storage and retrieval.
//
// ARCHITECTURE (read this before modifying):
//
// Bootstrap keys come from process environment (injected by systemd from GitHub Secrets):
//   - SUPABASE_URL: Database endpoint
//   - SUPABASE_SERVICE_KEY: Service role key (bypasses RLS, can read/write vault)
//   - VAULT_KEY: Master key for AES-GCM decryption
//
// All other secrets (GITHUB_TOKEN, API keys, etc.) are stored encrypted in the
// secrets_vault table and retrieved at runtime. They never touch the environment.
//
// WHY SERVICE_KEY (not anon key):
//   - The secrets_vault table has RLS enabled
//   - Only service_role can read/write (anon is blocked)
//   - This is intentional: prevents any compromised agent from dumping the vault
//   - SERVICE_KEY is only in root-only systemd override, not accessible to agents
//
// HOST PORTABILITY:
//   - The encryption salt is FIXED (not hostname-based)
//   - Same VAULT_KEY = same decryption on any host
//   - Pack up, move to new server, same secrets work
//
// DO NOT:
//   - Change key_env to SUPABASE_KEY (anon) - it won't work with RLS
//   - Add RLS policy for anon - defeats the security model
//   - Put keys in .env files - agents can read them
//   - Hardcode keys anywhere
//   - Change the salt - it will break all existing encrypted secrets
package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"github.com/vibepilot/governor/internal/db"
)

const (
	defaultCacheTTL  = 5 * time.Minute
	pbkdf2Iterations = 100000
	saltSize         = 16
	nonceSize        = 12
	keySize          = 32
	tableName        = "secrets_vault"
)

type Vault struct {
	db       db.Database
	cache    map[string]*cachedSecret
	mu       sync.RWMutex
	auditLog bool
	vaultKey []byte
}

type cachedSecret struct {
	value    string
	cachedAt time.Time
	maxAge   time.Duration
}

type AuditEntry struct {
	Operation string `json:"operation"`
	AgentID   string `json:"agent_id,omitempty"`
	Resource  string `json:"resource,omitempty"`
	KeyName   string `json:"key_name,omitempty"`
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
}

type SecretRecord struct {
	ID             string `json:"id"`
	KeyName        string `json:"key_name"`
	EncryptedValue string `json:"encrypted_value"`
	CreatedAt      string `json:"created_at"`
}

func New(database db.Database) *Vault {
	v := &Vault{
		db:       database,
		cache:    make(map[string]*cachedSecret),
		auditLog: true,
	}
	v.vaultKey = v.loadVaultKey()
	return v
}

func NewWithoutAudit(database db.Database) *Vault {
	v := &Vault{
		db:       database,
		cache:    make(map[string]*cachedSecret),
		auditLog: false,
	}
	v.vaultKey = v.loadVaultKey()
	return v
}

func (v *Vault) loadVaultKey() []byte {
	key := os.Getenv("VAULT_KEY")
	if key == "" {
		log.Printf("Vault: WARNING - VAULT_KEY not set, decryption will fail")
		return nil
	}
	return []byte(key)
}

func getMachineSalt() []byte {
	salt := sha256.Sum256([]byte("vibepilot-vault-portable-salt-v1"))
	return salt[:saltSize]
}

func (v *Vault) GetSecret(ctx context.Context, keyName string) (string, error) {
	v.mu.RLock()
	if cached, ok := v.cache[keyName]; ok {
		if time.Since(cached.cachedAt) < cached.maxAge {
			v.mu.RUnlock()
			v.logAudit(ctx, "vault_read", keyName, true, "cache_hit")
			return cached.value, nil
		}
	}
	v.mu.RUnlock()

	record, err := v.fetchFromDB(ctx, keyName)
	if err != nil {
		v.logAudit(ctx, "vault_read", keyName, false, err.Error())
		return "", fmt.Errorf("fetch secret %s: %w", keyName, err)
	}

	decrypted, err := v.decrypt(record.EncryptedValue)
	if err != nil {
		v.logAudit(ctx, "vault_read", keyName, false, "decrypt_failed")
		return "", fmt.Errorf("decrypt secret %s: %w", keyName, err)
	}

	v.mu.Lock()
	v.cache[keyName] = &cachedSecret{
		value:    decrypted,
		cachedAt: time.Now(),
		maxAge:   defaultCacheTTL,
	}
	v.mu.Unlock()

	v.logAudit(ctx, "vault_read", keyName, true, "success")
	return decrypted, nil
}

func (v *Vault) GetSecretNoCache(ctx context.Context, keyName string) (string, error) {
	record, err := v.fetchFromDB(ctx, keyName)
	if err != nil {
		v.logAudit(ctx, "vault_read", keyName, false, err.Error())
		return "", fmt.Errorf("fetch secret %s: %w", keyName, err)
	}

	decrypted, err := v.decrypt(record.EncryptedValue)
	if err != nil {
		v.logAudit(ctx, "vault_read", keyName, false, "decrypt_failed")
		return "", fmt.Errorf("decrypt secret %s: %w", keyName, err)
	}

	v.logAudit(ctx, "vault_read", keyName, true, "success_nocache")
	return decrypted, nil
}

func (v *Vault) fetchFromDB(ctx context.Context, keyName string) (*SecretRecord, error) {
	data, err := v.db.Query(ctx, tableName, map[string]any{
		"key_name": keyName,
		"limit":    1,
	})
	if err != nil {
		return nil, fmt.Errorf("db query: %w", err)
	}

	var records []SecretRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("secret not found: %s", keyName)
	}

	return &records[0], nil
}

func (v *Vault) decrypt(encrypted string) (string, error) {
	if v.vaultKey == nil {
		return "", fmt.Errorf("VAULT_KEY not configured")
	}

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

	key := deriveKey(string(v.vaultKey), salt)

	block, err := aes.NewCipher(key)
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

func Encrypt(plaintext, masterKey string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	key := deriveKey(masterKey, salt)

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

func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)
}

func (v *Vault) logAudit(ctx context.Context, operation, keyName string, allowed bool, reason string) {
	if !v.auditLog {
		return
	}

	entry := AuditEntry{
		Operation: operation,
		KeyName:   keyName,
		Allowed:   allowed,
		Reason:    reason,
	}

	details, _ := json.Marshal(entry)
	_, err := v.db.RPC(ctx, "log_security_audit", map[string]interface{}{
		"p_operation": operation,
		"p_key_name":  keyName,
		"p_allowed":   allowed,
		"p_reason":    reason,
	})
	if err != nil {
		log.Printf("Vault: failed to log audit: %v (entry: %s)", err, string(details))
	}
}

func (v *Vault) InvalidateCache(keyName string) {
	v.mu.Lock()
	delete(v.cache, keyName)
	v.mu.Unlock()
}

func (v *Vault) InvalidateAll() {
	v.mu.Lock()
	v.cache = make(map[string]*cachedSecret)
	v.mu.Unlock()
}

func (v *Vault) CacheStats() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := map[string]interface{}{
		"cached_keys": len(v.cache),
		"keys":        make([]string, 0, len(v.cache)),
	}

	for k := range v.cache {
		stats["keys"] = append(stats["keys"].([]string), k)
	}

	return stats
}

func GetEnvOrVault(ctx context.Context, v *Vault, keyName string) string {
	if val := os.Getenv(keyName); val != "" {
		return val
	}

	if v == nil {
		return ""
	}

	val, err := v.GetSecret(ctx, keyName)
	if err != nil {
		log.Printf("Vault: GetEnvOrVault failed for %s: %v", keyName, err)
		return ""
	}

	return val
}

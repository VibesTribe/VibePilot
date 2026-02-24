package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

type Vault struct {
	db       *db.DB
	cache    map[string]*cachedSecret
	mu       sync.RWMutex
	auditLog bool
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

const defaultCacheTTL = 5 * time.Minute

func New(database *db.DB) *Vault {
	return &Vault{
		db:       database,
		cache:    make(map[string]*cachedSecret),
		auditLog: true,
	}
}

func NewWithoutAudit(database *db.DB) *Vault {
	return &Vault{
		db:       database,
		cache:    make(map[string]*cachedSecret),
		auditLog: false,
	}
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
	path := fmt.Sprintf("secrets_vault?key_name=eq.%s&limit=1", keyName)
	data, err := v.db.REST(ctx, "GET", path, nil)
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
	vaultKey := os.Getenv("VAULT_KEY")
	if vaultKey == "" {
		return "", fmt.Errorf("VAULT_KEY not set")
	}

	keyBytes := deriveKey(vaultKey)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

func deriveKey(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
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

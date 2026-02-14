import os
from cryptography.fernet import Fernet
from supabase import create_client
from dotenv import load_dotenv
import base64, hashlib

load_dotenv()

_vault_instance = None


def get_api_key(key_name: str) -> str:
    """
    Retrieve an API key from vault.
    Runners should use this instead of os.getenv().

    Args:
        key_name: Name of the key (e.g., 'DEEPSEEK_API_KEY')

    Returns:
        The decrypted API key

    Raises:
        ValueError: If key not found in vault
    """
    global _vault_instance
    if _vault_instance is None:
        _vault_instance = VaultManager()

    value = _vault_instance.get_secret(key_name)
    if value is None:
        raise ValueError(f"Key '{key_name}' not found in vault")
    return value


def get_env_or_vault(key_name: str) -> str:
    """
    Get a value from vault, falling back to env var.
    Use for transition period or for bootstrap keys.
    """
    global _vault_instance

    env_value = os.getenv(key_name)
    if env_value:
        return env_value

    if _vault_instance is None:
        try:
            _vault_instance = VaultManager()
        except ValueError:
            return None

    return _vault_instance.get_secret(key_name) if _vault_instance else None


class VaultManager:
    def __init__(self):
        key = os.getenv("VAULT_KEY")
        if not key:
            raise ValueError("🛑 VAULT_KEY not found.")

        # Ensure valid key format
        if len(key) != 44:
            hashed = hashlib.sha256(key.encode()).digest()
            self.cipher = Fernet(base64.urlsafe_b64encode(hashed))
        else:
            self.cipher = Fernet(key)

        url = os.getenv("SUPABASE_URL")
        db_key = os.getenv("SUPABASE_KEY")
        self.client = create_client(url, db_key)

    def ingest_secret(self, key_name, raw_secret):
        encrypted_bytes = self.cipher.encrypt(raw_secret.encode())
        self.client.table("secrets_vault").upsert(
            {"key_name": key_name, "encrypted_value": encrypted_bytes.decode()}
        ).execute()
        print(f"✅ Secret for '{key_name}' locked in Vault.")

    def get_secret(self, key_name):
        response = (
            self.client.table("secrets_vault")
            .select("*")
            .eq("key_name", key_name)
            .execute()
        )
        if not response.data:
            return None
        decrypted_bytes = self.cipher.decrypt(
            response.data[0]["encrypted_value"].encode()
        )
        return decrypted_bytes.decode("utf-8")

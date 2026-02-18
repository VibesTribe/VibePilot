"""
Vault Manager - Secure access to secrets stored in Supabase.
All secrets encrypted at rest, decrypted on access.
"""

import os
from typing import Optional
from supabase import create_client
from cryptography.fernet import Fernet


class VaultManager:
    """Manages access to encrypted secrets in Supabase secrets_vault table."""
    
    def __init__(self):
        self.supabase_url = os.getenv('SUPABASE_URL')
        self.supabase_key = os.getenv('SUPABASE_KEY')
        self.vault_key = os.getenv('VAULT_KEY')
        
        if not all([self.supabase_url, self.supabase_key, self.vault_key]):
            raise ValueError("Missing required environment variables: SUPABASE_URL, SUPABASE_KEY, VAULT_KEY")
        
        self.client = create_client(self.supabase_url, self.supabase_key)
        self.cipher = Fernet(self.vault_key.encode())
    
    def get_secret(self, key_name: str) -> Optional[str]:
        """
        Retrieve and decrypt a secret from the vault.
        
        Args:
            key_name: Name of the secret to retrieve
            
        Returns:
            Decrypted secret value or None if not found
        """
        try:
            result = self.client.table('secrets_vault').select('encrypted_value').eq('key_name', key_name).execute()
            
            if not result.data:
                return None
            
            encrypted_value = result.data[0]['encrypted_value']
            decrypted_value = self.cipher.decrypt(encrypted_value.encode()).decode()
            return decrypted_value
            
        except Exception as e:
            print(f"Vault error retrieving {key_name}: {e}")
            return None
    
    def store_secret(self, key_name: str, value: str) -> bool:
        """
        Encrypt and store a secret in the vault.
        
        Args:
            key_name: Name of the secret
            value: Value to encrypt and store
            
        Returns:
            True if successful, False otherwise
        """
        try:
            encrypted_value = self.cipher.encrypt(value.encode()).decode()
            
            # Check if key exists
            existing = self.client.table('secrets_vault').select('id').eq('key_name', key_name).execute()
            
            if existing.data:
                # Update existing
                self.client.table('secrets_vault').update({'encrypted_value': encrypted_value}).eq('key_name', key_name).execute()
            else:
                # Insert new
                self.client.table('secrets_vault').insert({
                    'key_name': key_name,
                    'encrypted_value': encrypted_value
                }).execute()
            
            return True
            
        except Exception as e:
            print(f"Vault error storing {key_name}: {e}")
            return False


# Singleton instance
_vault_manager = None


def get_vault() -> VaultManager:
    """Get or create vault manager singleton."""
    global _vault_manager
    if _vault_manager is None:
        _vault_manager = VaultManager()
    return _vault_manager

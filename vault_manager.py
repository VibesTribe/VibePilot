import os
from cryptography.fernet import Fernet
from supabase import create_client
from dotenv import load_dotenv
import base64, hashlib

load_dotenv()

class VaultManager:
    def __init__(self):
        key = os.getenv("VAULT_KEY")
        if not key: raise ValueError("🛑 VAULT_KEY not found.")
        
        # Ensure valid key format
        if len(key) != 44:
            hashed = hashlib.sha256(key.encode()).digest()
            self.cipher = Fernet(base64.urlsafe_b64encode(hashed))
        else:
            self.cipher = Fernet(key)
            
        url = os.getenv("SUPABASE_URL")
        db_key = os.getenv("SUPABASE_KEY")
        self.client = create_client(url, db_key)

    def ingest_secret(self, provider_id, raw_secret):
        encrypted_bytes = self.cipher.encrypt(raw_secret.encode())
        self.client.table('secrets_vault').upsert({
            "provider_id": provider_id,
            "encrypted_secret": encrypted_bytes.decode(),
            "nonce": "fernet_token"
        }).execute()
        print(f"✅ Secret for '{provider_id}' locked in Vault.")

    def get_secret(self, provider_id):
        response = self.client.table('secrets_vault').select("*").eq('provider_id', provider_id).execute()
        if not response.data: return None
        decrypted_bytes = self.cipher.decrypt(response.data[0]['encrypted_secret'].encode())
        return decrypted_bytes.decode('utf-8')

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from vault_manager import VaultManager
from getpass import getpass

def main():
    vm = VaultManager()
    providers = ['deepseek', 'openai', 'glm', 'anthropic']
    print("🏛️  VibePilot Vault Ingestion")
    for provider in providers:
        key = getpass(f"   {provider.upper()}_KEY: ")
        if key: vm.ingest_secret(provider, key)

if __name__ == "__main__": main()

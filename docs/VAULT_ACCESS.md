# Vault Access Guide

## Key Locations

| Key Type | Location | Access Method |
|----------|----------|---------------|
| SUPABASE_URL | .env file | Direct read |
| SUPABASE_SERVICE_KEY | Vault | `vault_manager.py` |
| GITHUB_TOKEN | Vault | `vault_manager.py` |
| Other API keys | Vault | `vault_manager.py` |

## How to Access Vault from Python

```python
from vault_manager import VaultManager

vm = VaultManager()
key = vm.get_secret('SUPABASE_SERVICE_KEY')
```

## Example: Create Supabase Client

```python
import os
from supabase import create_client
from vault_manager import VaultManager

sb_url = os.environ.get('SUPABASE_URL', 'https://qtpdzsinvifkgpxyxlaz.supabase.co')
vm = VaultManager()
sb_key = vm.get_secret('SUPABASE_SERVICE_KEY')
sb = create_client(sb_url, sb_key)
```

## Files That Use Vault

- `core/orchestrator.py`
- `agents/supervisor.py`
- `agents/maintenance.py`
- `task_manager.py`

## Remember

- Supabase keys and vault key are in **GitHub secrets**
- All other keys are in **vault**
- Use `vault_manager.py` to access vault secrets

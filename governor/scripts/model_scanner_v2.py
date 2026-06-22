#!/usr/bin/env python3
"""
VibePilot Config-Driven Model Health Scanner v2

Reads provider_scanners.yaml and model_policy.yaml to:
1. Scan each provider for available models
2. Persist to model_catalog table
3. Health-check models and record in model_health_snapshots
4. Apply policy exclusions based on task requirements

Replaces the hardcoded daily_model_health.py
"""

import json
import os
import re
import sys
import subprocess
import time
import urllib.request
import urllib.error
import yaml
from datetime import datetime, timezone
from pathlib import Path

CONFIG_DIR = Path(__file__).parent.parent / "config"
PROVIDERS_FILE = CONFIG_DIR / "provider_scanners.yaml"
POLICY_FILE = CONFIG_DIR / "model_policy.yaml"

def psql_query(sql):
    """Run a psql query and return output."""
    r = subprocess.run(
        ['psql', '-d', 'vibepilot', '-t', '-A', '-c', sql],
        capture_output=True, text=True, timeout=10
    )
    return r.stdout.strip()

def psql_exec(sql):
    """Run a psql mutation."""
    r = subprocess.run(
        ['psql', '-d', 'vibepilot', '-c', sql],
        capture_output=True, text=True, timeout=10
    )
    return r.returncode == 0, r.stdout.strip()

def load_config():
    """Load provider and policy configs."""
    with open(PROVIDERS_FILE) as f:
        providers = yaml.safe_load(f)
    with open(POLICY_FILE) as f:
        policy = yaml.safe_load(f)
    return providers, policy

def load_env():
    """Load API keys from ~/.hermes/.env, fallback to process env."""
    env = {}
    env_file = Path.home() / ".hermes" / ".env"
    if env_file.exists():
        for line in env_file.read_text().splitlines():
            line = line.strip()
            if line and not line.startswith('#') and '=' in line:
                key, val = line.split('=', 1)
                env[key.strip()] = val.strip('"').strip("'")
    # Also inherit from process environment
    for key in ["GROQ_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
                "OPENROUTER_API_KEY", "NVIDIA_API_KEY", "ZAI_API_KEY",
                "DEEPSEEK_API_KEY", "SILICONFLOW_API_KEY"]:
        if key in os.environ and key not in env:
            env[key] = os.environ[key]
    return env

def api_request(url, method="GET", headers=None, body=None, timeout=15):
    """Make an HTTP request using curl (works in cron sandbox, unlike urllib)."""
    import subprocess as _sp
    cmd = ["curl", "-s", "--connect-timeout", str(timeout), "--max-time", str(timeout + 5)]
    cmd += ["-H", "User-Agent: VibePilot-Scanner/2.0"]
    cmd += ["-H", "Content-Type: application/json"]
    if headers:
        for k, v in headers.items():
            cmd += ["-H", f"{k}: {v}"]
    if method == "POST" and body:
        cmd += ["-d", json.dumps(body)]
        cmd.append("-X")
        cmd.append("POST")
    cmd.append(url)
    try:
        r = _sp.run(cmd, capture_output=True, text=True, timeout=timeout + 10)
        if r.returncode != 0:
            return None, f"curl exit {r.returncode}: {r.stderr.strip()[:500]}", r.returncode
        data = json.loads(r.stdout)
        return data, None, 200
    except json.JSONDecodeError as e:
        return None, f"JSON decode error: {e}", None
    except Exception as e:
        return None, str(e), None

def build_auth(config, env):
    """Build auth headers/params from provider config."""
    auth_type = config.get('auth_type', 'bearer')
    env_var = config.get('auth_env_var', '')
    api_key = env.get(env_var, '')
    
    if auth_type == 'bearer':
        return {'headers': {'Authorization': f'Bearer {api_key}'}} if api_key else {}
    elif auth_type == 'query_param':
        param = config.get('auth_param_name', 'key')
        return {'query_param': f'{param}={api_key}'} if api_key else {}
    return {}

def infer_capabilities(model_id, metadata_str, policy):
    """Infer capabilities from model ID and metadata using policy rules."""
    caps = set()
    
    # First try to get from metadata (OpenRouter returns architecture/capability info)
    try:
        meta = json.loads(metadata_str) if isinstance(metadata_str, str) else metadata_str
        arch = meta.get('architecture', {})
        
        # OpenRouter specific
        if 'modality' in meta:
            modalities = meta.get('modality', {})
            if 'vision' in str(modalities):
                caps.add('vision')
            if 'audio' in str(modalities):
                caps.add('audio')
    except:
        pass
    
    # Apply inference patterns from policy
    inference = policy.get('capability_inference', {})
    model_id_lower = model_id.lower()
    
    for cap_name, patterns in inference.items():
        for pattern in patterns:
            if re.search(pattern, model_id_lower):
                caps.add(cap_name)
    
    # Additional heuristics
    # Models with /vendor/ prefix on OpenRouter
    if any(x in model_id_lower for x in ['coder', 'code', 'devstral', 'codestral']):
        caps.add('code')
    if any(x in model_id_lower for x in ['-b-', 'instruct', 'chat']):
        if 'code' not in caps:
            # Instruct/chat models may or may not code - mark as text only
            caps.add('text')
    if any(x in model_id_lower for x in ['reasoning', 'think', 'r1']):
        caps.add('reasoning')
    if any(x in model_id_lower for x in ['vision', '-vl', 'multimodal']):
        caps.add('vision')
    if any(x in model_id_lower for x in ['tool', 'function', 'agent']):
        caps.add('tool_calling')
    
    # Default: if we can't infer anything, mark as text
    if not caps:
        caps.add('text')
    
    return sorted(caps)

def scan_provider(provider_name, config, env, policy):
    """Scan a single provider for models."""
    if not config.get('enabled', False):
        print(f"  {provider_name}: DISABLED, skipping")
        return []
    
    api_base = config['api_base']
    auth = build_auth(config, env)
    
    # Build list URL
    list_url = api_base + config.get('list_endpoint', '/models')
    if 'query_param' in auth:
        list_url += f"?{auth['query_param']}"
    
    headers = auth.get('headers', {})
    
    print(f"  {provider_name}: Scanning {list_url}...")
    data, err, code = api_request(list_url, headers=headers)
    
    if err:
        print(f"  {provider_name}: ERROR - {err[:200]}")
        return []
    
    # Extract models from response
    models_path = config.get('models_path', 'data')
    raw_models = data
    for key in models_path.split('.'):
        raw_models = raw_models.get(key, []) if isinstance(raw_models, dict) else []
    
    fm = config.get('field_mapping', {})
    parsed = []
    
    for m in raw_models:
        model_id = deep_get(m, fm.get('id', 'id'))
        if not model_id:
            continue
        
        # Clean up model ID
        if provider_name == 'google' and model_id.startswith('models/'):
            model_id = model_id.replace('models/', '')
        
        # Skip patterns
        skip = any(p in model_id for p in config.get('skip_patterns', []))
        if skip:
            continue
        
        is_free = any(ind in model_id for ind in config.get('free_indicators', []))
        if config.get('fixed_vendor') == 'google':
            is_free = True  # All Gemini models are free tier
        
        # Determine vendor
        vendor = config.get('fixed_vendor', provider_name)
        if config.get('vendor_from_id', False) and '/' in model_id:
            vendor = model_id.split('/')[0]
        
        context_length = deep_get(m, fm.get('context_length'))
        if context_length:
            try:
                context_length = int(context_length)
            except (ValueError, TypeError):
                context_length = None
        
        pricing_in = deep_get(m, fm.get('pricing_input'))
        pricing_out = deep_get(m, fm.get('pricing_output'))
        
        model_info = {
            'id': model_id,
            'name': deep_get(m, fm.get('name', 'id'), model_id),
            'provider': vendor,
            'connector_ids': [provider_name],
            'is_free': is_free,
            'context_length': context_length,
            'pricing_input': safe_float(pricing_in),
            'pricing_output': safe_float(pricing_out),
            'metadata': json.dumps(m, default=str)[:4000],
        }
        
        # Rate limits
        limits = config.get('default_free_limits', {})
        if limits:
            model_info['rate_limits'] = json.dumps(limits)
        
        # Infer capabilities
        meta_str = model_info.get('metadata', '{}')
        caps = infer_capabilities(model_id, meta_str, policy)
        if caps:
            model_info['capabilities'] = caps
        
        parsed.append(model_info)
    
    print(f"  {provider_name}: Found {len(parsed)} models")
    return parsed

def deep_get(obj, path, default=None):
    """Get a nested value from a dict using dot notation."""
    if not path or obj is None:
        return default
    keys = path.split('.')
    for key in keys:
        if isinstance(obj, dict):
            obj = obj.get(key)
        else:
            return default
        if obj is None:
            return default
    return obj

def safe_float(val):
    """Safely convert to float."""
    if val is None:
        return 0
    try:
        return float(val)
    except (ValueError, TypeError):
        return 0

def persist_models(models):
    """Upsert models into model_catalog."""
    upserted = 0
    for m in models:
        rate_limits = m.get('rate_limits', '{}')
        if isinstance(rate_limits, dict):
            rate_limits = json.dumps(rate_limits)
        
        metadata = m.get('metadata', '{}')
        if isinstance(metadata, dict):
            metadata = json.dumps(metadata)
        
        caps = m.get('capabilities', [])
        caps_sql = f"ARRAY{caps}::text[]" if caps else "model_catalog.capabilities"
        # Only overwrite capabilities if we have inferred ones and DB doesn't already have them
        caps_clause = f"""
            CASE WHEN array_length(model_catalog.capabilities, 1) > 0 
                 THEN model_catalog.capabilities 
                 ELSE {caps_sql} END
        """ if caps else "model_catalog.capabilities"
        
        sql = f"""
            INSERT INTO model_catalog (id, name, provider, connector_ids, status, 
                capabilities, is_free, context_length, pricing_input, pricing_output,
                rate_limits, last_scan_at, discovered_by, metadata, updated_at)
            VALUES (
                '{m["id"].replace("'", "''")}',
                '{m["name"].replace("'", "''")}',
                '{m["provider"].replace("'", "''")}',
                ARRAY['{m.get("connector_ids", ["unknown"])[0].replace("'", "''")}'],
                'active',
                ARRAY{caps}::text[],
                {m.get('is_free', True)},
                {m.get('context_length', 'NULL') or 'NULL'},
                {m.get('pricing_input', 0)},
                {m.get('pricing_output', 0)},
                '{rate_limits.replace("'", "''")}'::jsonb,
                NOW(),
                'scanner_v2',
                '{metadata.replace("'", "''")}'::jsonb,
                NOW()
            )
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name,
                provider = EXCLUDED.provider,
                connector_ids = EXCLUDED.connector_ids,
                capabilities = CASE WHEN array_length(model_catalog.capabilities, 1) > 0 
                    THEN model_catalog.capabilities ELSE EXCLUDED.capabilities END,
                is_free = EXCLUDED.is_free,
                context_length = COALESCE(EXCLUDED.context_length, model_catalog.context_length),
                pricing_input = EXCLUDED.pricing_input,
                pricing_output = EXCLUDED.pricing_output,
                rate_limits = EXCLUDED.rate_limits,
                last_scan_at = NOW(),
                discovered_by = 'scanner_v2',
                metadata = EXCLUDED.metadata,
                updated_at = NOW()
        """
        ok, _ = psql_exec(sql)
        if ok:
            upserted += 1
    return upserted

def health_check_model(model_id, provider_name, config, env):
    """Health-check a single model."""
    api_base = config['api_base']
    auth = build_auth(config, env)
    headers = auth.get('headers', {})
    
    health_ep = config.get('health_endpoint', '/chat/completions')
    health_ep = health_ep.replace('{model_id}', model_id)
    url = api_base + health_ep
    
    if 'query_param' in auth:
        url += f"?{auth['query_param']}"
    
    payload = config.get('health_payload', {})
    # Replace {model_id} in payload
    payload_str = json.dumps(payload).replace('{model_id}', model_id)
    payload = json.loads(payload_str)
    
    timeout = 15
    start = time.time()
    data, err, code = api_request(url, method="POST", headers=headers, body=payload, timeout=timeout)
    elapsed_ms = int((time.time() - start) * 1000)
    
    is_alive = err is None
    # 429 = rate limited but alive
    if code == 429:
        is_alive = True
    
    return is_alive, code, elapsed_ms, err

def record_health(model_id, is_alive, response_code, response_time_ms, error_detail):
    """Record health check result."""
    error_escaped = (error_detail or '').replace("'", "''")[:500]
    sql = f"""
        INSERT INTO model_health_snapshots 
            (model_id, is_alive, response_code, response_time_ms, error_detail, scanner_version)
        VALUES (
            '{model_id.replace("'", "''")}',
            {is_alive},
            {response_code or 'NULL'},
            {response_time_ms},
            {'NULL' if not error_detail else "'" + error_escaped + "'"},
            'v2'
        )
    """
    psql_exec(sql)
    
    # Update model_catalog health status
    if is_alive:
        psql_exec(f"""
            UPDATE model_catalog 
            SET last_healthy_at = NOW(), 
                consecutive_failures = 0,
                updated_at = NOW()
            WHERE id = '{model_id.replace("'", "''")}'
        """)
    else:
        psql_exec(f"""
            UPDATE model_catalog 
            SET consecutive_failures = consecutive_failures + 1,
                updated_at = NOW()
            WHERE id = '{model_id.replace("'", "''")}'
        """)

def apply_policy(policy):
    """Apply model_policy.yaml exclusions to model_catalog."""
    task_types = policy.get('task_types', {})
    global_excl = policy.get('global_exclusions', {})
    
    # Reset exclusions first
    psql_exec("UPDATE model_catalog SET excluded_from = '{}', exclusion_reason = NULL")
    
    excluded_count = 0
    
    for task_type, rules in task_types.items():
        required_caps = rules.get('required_capabilities', [])
        min_ctx = rules.get('min_context_length', 0)
        patterns = rules.get('excluded_patterns', [])
        
        # Build exclusion SQL
        conditions = []
        
        # Missing required capabilities
        for cap in required_caps:
            conditions.append(f"NOT ('{cap}' = ANY(capabilities))")
        
        # Context too small
        if min_ctx:
            conditions.append(f"(context_length IS NULL OR context_length < {min_ctx})")
        
        # Pattern exclusions
        pattern_conditions = []
        for pattern in patterns:
            pattern_conditions.append(f"id ILIKE '%{pattern}%'")
        
        # Global exclusions
        for pattern in global_excl.get('patterns', []):
            pattern_conditions.append(f"id ILIKE '%{pattern}%'")
        
        # Combine
        where_parts = []
        if conditions:
            where_parts.append(f"({' OR '.join(conditions)})")
        if pattern_conditions:
            where_parts.append(f"({' OR '.join(pattern_conditions)})")
        
        if not where_parts:
            continue
        
        where = ' OR '.join(where_parts)
        sql = f"""
            UPDATE model_catalog 
            SET excluded_from = array_cat(excluded_from, ARRAY['{task_type}']),
                exclusion_reason = 'Policy: excluded from {task_type}'
            WHERE status = 'active'
              AND ('{task_type}' = ANY(excluded_from)) IS NOT TRUE
              AND ({where})
            RETURNING id
        """
        r = psql_query(sql)
        count = len([l for l in r.split('\n') if l]) if r else 0
        excluded_count += count
        if count:
            print(f"  Excluded {count} models from {task_type}")
    
    # Auto-exclude models with too many consecutive failures
    max_failures = global_excl.get('max_consecutive_failures', 5)
    r = psql_query(f"""
        UPDATE model_catalog
        SET status = 'unhealthy',
            status_reason = 'Too many consecutive failures (>={max_failures})',
            updated_at = NOW()
        WHERE consecutive_failures >= {max_failures}
          AND status = 'active'
        RETURNING id
    """)
    unhealthy = len([l for l in r.split('\n') if l]) if r else 0
    if unhealthy:
        print(f"  Marked {unhealthy} models as unhealthy (too many failures)")
    
    print(f"  Total exclusions applied: {excluded_count}")

def main():
    print(f"=== VibePilot Model Scanner v2 ===")
    print(f"Started: {datetime.now(timezone.utc).isoformat()}")
    
    # Load configs
    providers_config, policy = load_config()
    env = load_env()
    
    providers = providers_config.get('providers', {})
    
    # Phase 1: Scan providers
    print("\n--- Phase 1: Scanning Providers ---")
    all_models = []
    for name, config in providers.items():
        models = scan_provider(name, config, env, policy)
        all_models.extend(models)
    
    print(f"\nTotal models discovered: {len(all_models)}")
    
    # Phase 2: Persist to DB
    print("\n--- Phase 2: Persisting to DB ---")
    upserted = persist_models(all_models)
    print(f"Upserted {upserted}/{len(all_models)} models")
    
    # Phase 3: Health check a sample
    print("\n--- Phase 3: Health Checking ---")
    scanner_config = policy.get('scanner', {})
    batch_size = scanner_config.get('health_check_batch_size', 10)
    
    # Get models that need health checking (oldest first)
    r = psql_query(f"""
        SELECT id, provider FROM model_catalog
        WHERE status = 'active'
        ORDER BY last_healthy_at ASC NULLS FIRST
        LIMIT {batch_size}
    """)
    
    if r:
        models_to_check = []
        for line in r.split('\n'):
            if '|' in line:
                parts = line.split('|')
                models_to_check.append((parts[0], parts[1]))
        
        print(f"Checking {len(models_to_check)} models...")
        
        for model_id, provider in models_to_check:
            # Match model to its scanner provider config
            # Strategy: try connector_ids first, then vendor_from_id, then fallback
            prov_config = None
            prov_name = None
            
            # Get connector_ids from DB for this model
            conn_r = psql_query(f"SELECT array_to_string(connector_ids, ',') FROM model_catalog WHERE id = '{model_id.replace(chr(39), chr(39)+chr(39))}'")
            connectors = conn_r.split(',') if conn_r else []
            
            # Try matching by connector
            for cname in connectors:
                if cname in providers and providers[cname].get('enabled'):
                    prov_config = providers[cname]
                    prov_name = cname
                    break
            
            # Fallback: match by provider field
            if not prov_config:
                for pname, pconf in providers.items():
                    if pconf.get('fixed_vendor') == provider and pconf.get('enabled'):
                        prov_config = pconf
                        prov_name = pname
                        break
            
            # Fallback: use openrouter for any model with / in ID
            if not prov_config and '/' in model_id:
                if 'openrouter' in providers and providers['openrouter'].get('enabled'):
                    prov_config = providers['openrouter']
                    prov_name = 'openrouter'
            
            if prov_config:
                is_alive, code, ms, err = health_check_model(model_id, provider, prov_config, env)
                status = "OK" if is_alive else f"FAIL ({err[:60]})"
                print(f"  {model_id[:50]:50s} {status} ({ms}ms)")
                record_health(model_id, is_alive, code, ms, err)
                time.sleep(1)  # Rate limit between checks
            else:
                print(f"  {model_id}: No provider config for health check")
    
    # Phase 4: Apply policy
    print("\n--- Phase 4: Applying Policy ---")
    apply_policy(policy)
    
    # Summary
    print("\n=== Summary ===")
    total = psql_query("SELECT count(*) FROM model_catalog WHERE status = 'active'")
    viable = psql_query("""
        SELECT count(*) FROM model_catalog 
        WHERE status = 'active' 
          AND 'code' = ANY(capabilities)
          AND ('code_generation' = ANY(excluded_from)) IS NOT TRUE
    """)
    unhealthy = psql_query("SELECT count(*) FROM model_catalog WHERE status = 'unhealthy'")
    
    print(f"Active: {total}")
    print(f"Viable for code: {viable}")
    print(f"Unhealthy: {unhealthy}")
    print(f"Completed: {datetime.now(timezone.utc).isoformat()}")

if __name__ == '__main__':
    main()

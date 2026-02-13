import requests
import os
from dotenv import load_dotenv

load_dotenv()

KIMI_KEY = os.getenv("KIMI_KEY")
GLM_KEY = os.getenv("GLM_KEY")

# Test function
def test_provider(name, url, key, models):
    print(f"\n--- Testing {name} ---")
    for model in models:
        headers = {"Authorization": f"Bearer {key}", "Content-Type": "application/json"}
        payload = {"model": model, "messages": [{"role": "user", "content": "Hi"}], "stream": False}
        try:
            r = requests.post(url, headers=headers, json=payload, timeout=10)
            if r.status_code == 200:
                print(f"✅ SUCCESS: {name} works with model '{model}'")
                return model # Stop on first success
            else:
                err = r.json().get('error', {}).get('message', r.text)
                print(f"❌ Model '{model}' FAILED: {err}")
        except Exception as e:
            print(f"❌ Model '{model}' CRASHED: {e}")
    return None

# Scan GLM
glm_url = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
glm_models = ["glm-4", "glm-4-plus", "glm-4-flash", "glm-4-air", "glm-4-0520", "chatglm3-6b"]
working_glm = test_provider("GLM", glm_url, GLM_KEY, glm_models)

# Scan Kimi
kimi_url = "https://api.moonshot.cn/v1/chat/completions"
kimi_models = ["moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"]
working_kimi = test_provider("Kimi", kimi_url, KIMI_KEY, kimi_models)

print("\n" + "="*30)
if working_kimi:
    print(f"✅ WINNER: Use KIMI with model '{working_kimi}'")
elif working_glm:
    print(f"✅ WINNER: Use GLM with model '{working_glm}'")
else:
    print("❌ NO WORKING COMBINATION FOUND. Check Keys.")

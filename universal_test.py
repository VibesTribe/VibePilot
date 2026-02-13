import os
from dotenv import load_dotenv
from openai import OpenAI

load_dotenv()

def test(name, key, url, model):
    print(f"Testing {name}...")
    try:
        client = OpenAI(api_key=key, base_url=url)
        resp = client.chat.completions.create(
            model=model,
            messages=[{"role": "user", "content": "Say OK"}],
            timeout=10
        )
        print(f"SUCCESS: {resp.choices[0].message.content}")
    except Exception as e:
        print(f"FAILED: {e}")

print("--- TESTING HANDS (v2) ---")
test("DeepSeek", os.getenv("DEEPSEEK_API_KEY"), "https://api.deepseek.com", "deepseek-chat")
# CHANGED: moonshot-v1-8k is usually the right endpoint for Kimi
test("Kimi", os.getenv("KIMI_API_KEY"), "https://api.moonshot.cn/v1", "moonshot-v1-8k")
# CHANGED: glm-4-flash is the standard model (glm-4 might be premium only)
test("GLM", os.getenv("GLM_API_KEY"), "https://open.bigmodel.cn/api/paas/v4/", "glm-4-flash")

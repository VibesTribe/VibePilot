import os
from dotenv import load_dotenv
from openai import OpenAI

load_dotenv()

def test(model_name):
    print(f"Testing {model_name} (Standard)... ")
    try:
        client = OpenAI(api_key=os.getenv("GLM_API_KEY"), base_url="https://open.bigmodel.cn/api/paas/v4/")
        resp = client.chat.completions.create(
            model=model_name,
            messages=[{"role": "user", "content": "Say OK"}],
            timeout=10
        )
        print(f"SUCCESS: {resp.choices[0].message.content}")
    except Exception as e:
        print(f"FAILED: {e}")

print("--- TESTING STANDARD SUBSCRIPTION MODELS ---")
# These are the models typically included in Pro plans
test("glm-4")
test("glm-4-flash")
test("glm-4-air")

import os
from dotenv import load_dotenv
from openai import OpenAI

load_dotenv()

def test(model_name):
    print(f"Testing {model_name}...")
    try:
        client = OpenAI(api_key=os.getenv("GLM_API_KEY"), base_url="https://open.bigmodel.cn/api/paas/v4/")
        resp = client.chat.completions.create(
            model=model_name,
            messages=[{"role": "user", "content": "Say OK"}],
            timeout=10
        )
        print(f"SUCCESS: {resp.choices[0].message.content}\n")
    except Exception as e:
        print(f"FAILED: {e}\n")

print("--- TESTING GLM 5 & 4.7 ---")
test("glm-4.5")
test("glm-4.7")
test("glm-5")

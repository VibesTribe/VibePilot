import os
from dotenv import load_dotenv
from openai import OpenAI

load_dotenv()

key = os.getenv("GLM_API_KEY")
url = "https://open.bigmodel.cn/api/paas/v4/"

print("Connecting to GLM to see available models...")
try:
    client = OpenAI(api_key=key, base_url=url)
    models = client.models.list()
    
    print(f"\n--- FOUND {len(models.data)} MODELS ---")
    for m in models.data:
        # Only show the GLM models
        if "glm" in m.id.lower():
            print(m.id)
except Exception as e:
    print(f"ERROR: {e}")

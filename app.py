import gradio as gr
import os
import requests
from dotenv import load_dotenv

load_dotenv()

DS_KEY = os.getenv("DEEPSEEK_KEY")
GLM_KEY = os.getenv("GLM_KEY")
KIMI_KEY = os.getenv("KIMI_KEY")

def chat_with_api(message, history, provider):
    provider = provider.strip()
    url, model, key, cost_display = "", "", "", ""
    
    # --- FIX: Z.AI (Standard ZhipuAI Endpoint) ---
    if provider == "GLM (Coding Plan)":
        url = "https://open.bigmodel.cn/api/paas/v4/chat/completions" # STANDARD ENDPOINT
        model = "glm-4.6" # 2026 Model
        key = GLM_KEY
        cost_display = "💎 GLM Coding Plan"
    
    # --- FIX: Kimi (Standard Moonshot Endpoint) ---
    elif provider == "Kimi (Subscription)":
        url = "https://api.moonshot.cn/v1/chat/completions" # STANDARD ENDPOINT
        model = "moonshot-v1-auto" # Auto Model
        key = KIMI_KEY
        cost_display = "💎 Kimi Subscription"

    else:
        url = "https://api.deepseek.com/v1/chat/completions"
        model = "deepseek-chat"
        key = DS_KEY
        cost_display = "💸 DeepSeek"

    if not key or key.startswith("PASTE"):
        return "❌ Missing API Key."

    messages = [
        {"role": "system", "content": "You are a helpful assistant. Reply in English."},
        {"role": "user", "content": message}
    ]

    headers = {"Authorization": f"Bearer {key}", "Content-Type": "application/json"}
    payload = {"model": model, "messages": messages, "stream": False}
    
    try:
        r = requests.post(url, headers=headers, json=payload, timeout=20)
        if r.status_code == 200:
            return r.json()["choices"][0]["message"]["content"]
        else:
            return f"❌ Error: {r.text}"
    except Exception as e:
        return f"❌ Connection Error: {e}"

with gr.Blocks() as demo:
    gr.Markdown("# 🧠 VibePilot (Corrected)")
    provider = gr.Radio(
        ["GLM (Coding Plan)", "Kimi (Subscription)", "DeepSeek"],
        value="GLM (Coding Plan)",
        label="Provider"
    )
    chat = gr.ChatInterface(fn=chat_with_api, additional_inputs=[provider])
    demo.launch(server_name="0.0.0.0", server_port=8081)

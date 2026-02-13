import streamlit as st
import os
import requests
from supabase import create_client
from dotenv import load_dotenv

load_dotenv()
client = create_client(os.getenv("SUPABASE_URL"), os.getenv("SUPABASE_KEY"))

# --- AUTH ---
if "password_correct" not in st.session_state:
    st.text_input("Password", type="password", on_change=lambda: st.session_state.update({"password_correct": st.session_state.password == st.secrets.password}), key="password")
    if st.session_state.get("password_correct"): del st.session_state.password
    st.stop()

# --- SETUP ---
st.set_page_config(page_title="VibePilot", page_icon="🏛️")
st.title("🏛️ VibePilot Command Center")

# --- CHAT INTERFACE ---
if "messages" not in st.session_state: st.session_state.messages = []

for msg in st.session_state.messages:
    with st.chat_message(msg["role"]): st.markdown(msg["content"])

if prompt := st.chat_input("Talk to Vibes..."):
    st.session_state.messages.append({"role": "user", "content": prompt})
    with st.chat_message("user"): st.markdown(prompt)
    
    # Send to DB
    client.table('chat_queue').insert({"role": "user", "content": prompt}).execute()
    
    # Wait for reply (Simulated)
    with st.chat_message("assistant"):
        with st.spinner("Vibes is thinking..."):
            time.sleep(2) # Placeholder
            reply = "I received your message."
        st.markdown(reply)
    
    st.session_state.messages.append({"role": "assistant", "content": reply})

# --- TTS HOOK ---
if st.button("🔊 Read Last Message"):
    last_msg = st.session_state.messages[-1]["content"]
    # Placeholder for Kokoro TTS call on localhost
    # tts_resp = requests.post("http://localhost:8888/tts", json={"text": last_msg})
    # if tts_resp.ok: st.audio(tts_resp.content)
    st.info("TTS Service would play this audio.")

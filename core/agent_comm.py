"""
Agent Communication Module - Real-time messaging between GLM-5 and Kimi
Uses Supabase Realtime (WebSocket) for instant push-based communication.
"""

import os
import json
import threading
import time
from typing import Callable, Optional, Dict, Any
from datetime import datetime

try:
    from supabase import create_client
    from vault_manager import get_api_key
except ImportError:
    pass


class AgentComm:
    """Real-time agent-to-agent messaging via Supabase Realtime."""
    
    def __init__(self, agent_name: str, on_message: Optional[Callable] = None):
        self.agent_name = agent_name
        self.on_message = on_message
        self._client = None
        self._service_client = None
        self._channel = None
        self._running = False
        self._listener_thread = None
        self._poll_interval = 2
        
    def _get_client(self):
        """Get anon key client for subscriptions."""
        if self._client is None:
            url = os.getenv("SUPABASE_URL")
            key = os.getenv("SUPABASE_KEY")
            if url and key:
                self._client = create_client(url, key)
        return self._client
    
    def _get_service_client(self):
        """Get service key client for inserts."""
        if self._service_client is None:
            url = os.getenv("SUPABASE_URL")
            try:
                key = get_api_key("SUPABASE_SERVICE_KEY")
                if url and key:
                    self._service_client = create_client(url, key)
            except:
                key = os.getenv("SUPABASE_SERVICE_KEY")
                if url and key:
                    self._service_client = create_client(url, key)
        return self._service_client or self._get_client()
    
    def send(self, to_agent: str, content: Dict[str, Any], message_type: str = "chat") -> Optional[str]:
        """Send message to another agent instantly."""
        client = self._get_service_client()
        if not client:
            print(f"[AgentComm] No client available")
            return None
        
        try:
            result = client.table("agent_messages").insert({
                "from_agent": self.agent_name,
                "to_agent": to_agent,
                "message_type": message_type,
                "content": content
            }).execute()
            
            if result.data:
                msg_id = result.data[0].get("id")
                print(f"[AgentComm] Sent {message_type} to {to_agent}: {content.get('text', '')[:50]}...")
                return msg_id
        except Exception as e:
            print(f"[AgentComm] Send failed: {e}")
        return None
    
    def get_unread(self) -> list:
        """Pull unread messages (fallback if realtime fails)."""
        client = self._get_service_client()
        if not client:
            return []
        
        try:
            result = client.table("agent_messages")\
                .select("*")\
                .eq("to_agent", self.agent_name)\
                .is_("read_at", "null")\
                .order("created_at")\
                .execute()
            return result.data or []
        except Exception as e:
            print(f"[AgentComm] Get unread failed: {e}")
            return []
    
    def mark_read(self, message_id: str) -> bool:
        """Mark a message as read."""
        client = self._get_service_client()
        if not client:
            return False
        
        try:
            client.table("agent_messages")\
                .update({"read_at": datetime.utcnow().isoformat()})\
                .eq("id", message_id)\
                .execute()
            return True
        except Exception as e:
            print(f"[AgentComm] Mark read failed: {e}")
            return False
    
    def start_listening(self, poll_mode: bool = True):
        """
        Start listening for messages.
        
        poll_mode=True: Use polling (reliable fallback)
        poll_mode=False: Try WebSocket (may not work in all environments)
        """
        if poll_mode:
            self._start_polling()
        else:
            self._start_realtime()
    
    def _start_polling(self):
        """Start polling for messages in background thread."""
        if self._running:
            return
        
        self._running = True
        self._listener_thread = threading.Thread(target=self._poll_loop, daemon=True)
        self._listener_thread.start()
        print(f"[AgentComm] {self.agent_name} started polling for messages")
    
    def _poll_loop(self):
        """Polling loop - checks for new messages every N seconds."""
        last_check = datetime.utcnow()
        
        while self._running:
            try:
                client = self._get_service_client()
                if client:
                    result = client.table("agent_messages")\
                        .select("*")\
                        .eq("to_agent", self.agent_name)\
                        .is_("read_at", "null")\
                        .gt("created_at", last_check.isoformat())\
                        .order("created_at")\
                        .execute()
                    
                    for msg in result.data or []:
                        last_check = datetime.utcnow()
                        if self.on_message:
                            self.on_message(msg)
                        else:
                            self._default_handler(msg)
                        
            except Exception as e:
                print(f"[AgentComm] Poll error: {e}")
            
            time.sleep(self._poll_interval)
    
    def _start_realtime(self):
        """Try to start WebSocket realtime subscription."""
        client = self._get_client()
        if not client:
            print("[AgentComm] No client for realtime")
            self._start_polling()
            return
        
        try:
            # Try realtime subscription
            self._channel = client.channel('agent-messages')
            self._channel.on_postgres_change(
                'INSERT',
                schema='public',
                table='agent_messages',
                filter=f'to_agent=eq.{self.agent_name}',
                callback=self._realtime_callback
            ).subscribe()
            print(f"[AgentComm] {self.agent_name} subscribed to realtime")
        except Exception as e:
            print(f"[AgentComm] Realtime failed, falling back to polling: {e}")
            self._start_polling()
    
    def _realtime_callback(self, payload):
        """Handle realtime message."""
        msg = payload.get('record', payload)
        if self.on_message:
            self.on_message(msg)
        else:
            self._default_handler(msg)
    
    def _default_handler(self, msg: Dict):
        """Default message handler - just print."""
        from_agent = msg.get('from_agent', 'unknown')
        content = msg.get('content', {})
        text = content.get('text', str(content))
        msg_type = msg.get('message_type', 'chat')
        
        print(f"\n{'='*50}")
        print(f"📨 NEW MESSAGE from {from_agent} [{msg_type}]")
        print(f"{'='*50}")
        print(text[:500])
        print(f"{'='*50}\n")
        
        # Mark as read
        self.mark_read(msg.get('id'))
    
    def stop(self):
        """Stop listening."""
        self._running = False
        if self._channel:
            try:
                self._channel.unsubscribe()
            except:
                pass
        print(f"[AgentComm] {self.agent_name} stopped")
    
    def broadcast(self, content: Dict[str, Any], message_type: str = "alert") -> int:
        """Broadcast to all agents (to_agent = 'all')."""
        return self.send("all", content, message_type) is not None


# Convenience functions for quick usage
_comm_instance: Optional[AgentComm] = None


def init(agent_name: str, on_message: Optional[Callable] = None) -> AgentComm:
    """Initialize global comm instance."""
    global _comm_instance
    _comm_instance = AgentComm(agent_name, on_message)
    return _comm_instance


def send(to_agent: str, content: Dict[str, Any], message_type: str = "chat") -> Optional[str]:
    """Send using global instance."""
    if _comm_instance:
        return _comm_instance.send(to_agent, content, message_type)
    return None


def get_unread() -> list:
    """Get unread using global instance."""
    if _comm_instance:
        return _comm_instance.get_unread()
    return []


def start_listening():
    """Start listening using global instance."""
    if _comm_instance:
        _comm_instance.start_listening(poll_mode=True)


if __name__ == "__main__":
    # Test the module
    import sys
    
    agent = sys.argv[1] if len(sys.argv) > 1 else "test-agent"
    
    def handle(msg):
        print(f"Received: {msg}")
    
    comm = AgentComm(agent, on_message=handle)
    
    if len(sys.argv) > 2 and sys.argv[2] == "send":
        to = sys.argv[3] if len(sys.argv) > 3 else "other"
        comm.send(to, {"text": "Test message from " + agent})
    else:
        print(f"Listening as {agent}... (Ctrl+C to stop)")
        comm.start_listening()
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            comm.stop()

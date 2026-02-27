#!/usr/bin/env python3
"""
VibePilot Orchestrator Service Entry Point

This is the main entry point for running VibePilot as a service.
It starts the concurrent orchestrator which:
- Watches the task queue
- Dispatches tasks to available runners
- Processes supervisor reviews
- Tracks usage and rate limits
- Learns from task outcomes

Run directly: python run_orchestrator.py
Run as service: systemd runs this automatically on boot
"""

import os
import sys
import signal
import logging
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(name)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler(
            os.path.join(os.path.dirname(__file__), "logs", "orchestrator.log")
        ),
    ],
)
logger = logging.getLogger("VibePilot.Service")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    logger.error("Missing SUPABASE_URL or SUPABASE_KEY environment variables")
    sys.exit(1)

orchestrator = None


def signal_handler(signum, frame):
    """Handle shutdown signals gracefully."""
    global orchestrator
    logger.info(f"Received signal {signum}, shutting down...")
    if orchestrator:
        orchestrator.stop()
    sys.exit(0)


def main():
    global orchestrator

    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    logger.info("=" * 60)
    logger.info("VIBEPILOT ORCHESTRATOR SERVICE")
    logger.info("=" * 60)

    from core.orchestrator import ConcurrentOrchestrator

    orchestrator = ConcurrentOrchestrator()

    status = orchestrator.get_status()
    logger.info(f"Max workers: {status['max_workers']}")
    logger.info(
        f"Available runners: {status['available_runners']}/{status['total_runners']}"
    )
    logger.info(f"Active tasks: {status['active_tasks']}")
    logger.info(f"Pending reviews: {status['pending_reviews']}")

    logger.info("Starting orchestrator loop (Ctrl+C to stop)...")
    logger.info("=" * 60)

    try:
        orchestrator.start()
    except KeyboardInterrupt:
        logger.info("Interrupted by user")
    except Exception as e:
        logger.error(f"Orchestrator crashed: {e}", exc_info=True)
        sys.exit(1)
    finally:
        if orchestrator:
            orchestrator.stop()

    logger.info("Orchestrator stopped cleanly")
    sys.exit(0)


if __name__ == "__main__":
    main()

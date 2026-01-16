#!/usr/bin/env python3
"""
Jenkins job status monitor daemon with macOS notifications
Monitors multiple jobs concurrently in the background

Usage:
    python3 jenkins-monitor.py start [job_url1] [job_url2] ...
    python3 jenkins-monitor.py stop
    python3 jenkins-monitor.py status
    python3 jenkins-monitor.py --config config.json start

Requires: JENKINS_TOKEN environment variable
"""

import os
import sys
import time
import json
import signal
import logging
import requests
import subprocess
import threading
from pathlib import Path
from typing import Dict, Set
from datetime import datetime

# Colors for terminal output
GREEN = "\033[92m"
YELLOW = "\033[93m"
RED = "\033[91m"
ENDC = "\033[0m"

# Daemon settings
PID_FILE = os.path.expanduser("~/.jenkins_monitor.pid")
LOG_FILE = os.path.expanduser("~/.jenkins_monitor.log")
CONFIG_FILE = os.path.expanduser("~/.jenkins_monitor_jobs.json")


def setup_logging():
    """Configure logging to file and console"""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - %(message)s",
        handlers=[
            logging.FileHandler(LOG_FILE),
            logging.StreamHandler(),
        ],
    )
    return logging.getLogger(__name__)


logger = setup_logging()


def send_macos_notification(title, message):
    """Send macOS system notification"""
    cmd = f"terminal-notifier -message '{message}' -title '{title}' -sound ping"
    subprocess.run(cmd, shell=True, check=False)


def get_job_status(jenkins_url, token):
    """
    Get job status from Jenkins API
    Returns (is_building, result) or (None, None) if failed
    """
    api_url = jenkins_url.rstrip("/") + "/api/json?tree=building,result"
    headers = {"Authorization": f"Basic {token}"}

    try:
        response = requests.get(api_url, headers=headers)
        response.raise_for_status()
        data = response.json()
        return data.get("building"), data.get("result")
    except requests.RequestException as e:
        logger.error(f"Error connecting to Jenkins: {e}")
        return None, None


def monitor_job(job_url: str, token: str, job_states: Dict):
    """Monitor a single job in a separate thread"""
    job_name = job_url.split("/job/")[-1].rstrip("/")
    logger.info(f"Started monitoring: {job_name}")

    last_result = None

    while job_states.get(job_url, True):  # Continue while job_url is in dict
        building, result = get_job_status(job_url, token)

        if building is None:
            time.sleep(30)
            continue

        # Track state changes
        if not building and result != last_result:
            status = result if result else "UNKNOWN"
            logger.info(f"Build finished: {job_name} - Status: {status}")
            send_macos_notification(
                "Jenkins Job Completed",
                f"Job: {job_name}\nStatus: {status}",
            )
            last_result = result

            # Remove job from monitoring list
            job_states[job_url] = False
            jobs = load_config()
            jobs.discard(job_url)
            save_config(jobs)
            logger.info(f"Removed {job_name} from monitoring (build completed)")
            break

        time.sleep(10)

    logger.info(f"Stopped monitoring: {job_name}")


def load_config() -> Set[str]:
    """Load job URLs from config file"""
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE) as f:
                data = json.load(f)
                return set(data.get("jobs", []))
        except json.JSONDecodeError:
            logger.error(f"Invalid JSON in {CONFIG_FILE}")
    return set()


def save_config(jobs: Set[str]):
    """Save job URLs to config file"""
    with open(CONFIG_FILE, "w") as f:
        json.dump({"jobs": list(jobs)}, f, indent=2)


def start_daemon(job_urls: list = None):
    """Start the daemon"""
    # Check if already running
    if os.path.exists(PID_FILE):
        try:
            with open(PID_FILE) as f:
                pid = int(f.read().strip())
            os.kill(pid, 0)  # Check if process exists
            logger.error(f"Daemon already running (PID: {pid})")
            return False
        except (ProcessLookupError, ValueError):
            os.remove(PID_FILE)

    token = os.environ.get("JENKINS_TOKEN")
    if not token:
        logger.error("JENKINS_TOKEN environment variable not set!")
        return False

    # Collect job URLs
    jobs = load_config()
    if job_urls:
        jobs.update(job_urls)

    if not jobs:
        logger.error("No job URLs provided or configured")
        return False

    # Save PID
    with open(PID_FILE, "w") as f:
        f.write(str(os.getpid()))

    logger.info(f"Jenkins monitor daemon started (PID: {os.getpid()})")
    logger.info(f"Monitoring {len(jobs)} job(s)")

    # Store job states (thread-safe dict)
    job_states = {job: True for job in jobs}
    threads = []

    # Start monitoring thread for each job
    for job_url in jobs:
        thread = threading.Thread(
            target=monitor_job, args=(job_url, token, job_states), daemon=True
        )
        thread.start()
        threads.append(thread)

    # Handle graceful shutdown
    def signal_handler(sig, frame):
        logger.info("Shutdown signal received")
        for job in job_states:
            job_states[job] = False
        for thread in threads:
            thread.join(timeout=2)
        os.remove(PID_FILE)
        sys.exit(0)

    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    # Keep daemon running
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        signal_handler(None, None)


def stop_daemon():
    """Stop the daemon"""
    if not os.path.exists(PID_FILE):
        print(f"{YELLOW}Daemon not running{ENDC}")
        return

    try:
        with open(PID_FILE) as f:
            pid = int(f.read().strip())
        os.kill(pid, 15)  # SIGTERM
        logger.info(f"Sent SIGTERM to daemon (PID: {pid})")
        time.sleep(1)
        os.remove(PID_FILE)
        print(f"{GREEN}Daemon stopped{ENDC}")
    except (ProcessLookupError, ValueError, FileNotFoundError):
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
        logger.warning("Could not stop daemon")


def get_status():
    """Get daemon status"""
    if not os.path.exists(PID_FILE):
        print(f"{RED}Daemon not running{ENDC}")
        return

    try:
        with open(PID_FILE) as f:
            pid = int(f.read().strip())
        os.kill(pid, 0)
        print(f"{GREEN}Daemon running (PID: {pid}){ENDC}")

        jobs = load_config()
        print(f"Monitoring {len(jobs)} job(s):")
        for job in jobs:
            print(f"  - {job}")

        if os.path.exists(LOG_FILE):
            print(f"\nRecent logs (from {LOG_FILE}):")
            with open(LOG_FILE) as f:
                lines = f.readlines()[-10:]
                for line in lines:
                    print(f"  {line.rstrip()}")

    except ProcessLookupError:
        print(f"{RED}Daemon not running (stale PID file){ENDC}")
        os.remove(PID_FILE)


def add_job(job_url: str):
    """Add a job to monitor"""
    jobs = load_config()
    jobs.add(job_url)
    save_config(jobs)
    print(f"{GREEN}Added job: {job_url}{ENDC}")
    logger.info(f"Job added: {job_url}")


def remove_job(job_url: str):
    """Remove a job from monitoring"""
    jobs = load_config()
    if job_url in jobs:
        jobs.remove(job_url)
        save_config(jobs)
        print(f"{GREEN}Removed job: {job_url}{ENDC}")
        logger.info(f"Job removed: {job_url}")
    else:
        print(f"{YELLOW}Job not found: {job_url}{ENDC}")


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} {{start|stop|status|add|remove}} [job_urls...]")
        sys.exit(1)

    command = sys.argv[1]

    if command == "start":
        # Start with optional job URLs
        job_urls = sys.argv[2:] if len(sys.argv) > 2 else None
        start_daemon(job_urls)
    elif command == "stop":
        stop_daemon()
    elif command == "status":
        get_status()
    elif command == "add":
        if len(sys.argv) < 3:
            print("Usage: add <job_url>")
            sys.exit(1)
        add_job(sys.argv[2])
    elif command == "remove":
        if len(sys.argv) < 3:
            print("Usage: remove <job_url>")
            sys.exit(1)
        remove_job(sys.argv[2])
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        logger.info("Interrupted by user")
        sys.exit(0)

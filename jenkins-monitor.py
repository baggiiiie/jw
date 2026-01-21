#!/usr/bin/env python3
"""
Jenkins job status monitor daemon with macOS notifications
Monitors multiple jobs concurrently in the background

Usage:
    python3 jenkins-monitor.py add <job_url>
    python3 jenkins-monitor.py remove <job_url>
    python3 jenkins-monitor.py stop
    python3 jenkins-monitor.py status
    python3 jenkins-monitor.py logs

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
from typing import Dict, Set

# --- Constants & Globals ---

# Colors for terminal output
GREEN = "\033[92m"
YELLOW = "\033[93m"
RED = "\033[91m"
ENDC = "\033[0m"

# File paths
PID_FILE = os.path.expanduser("~/.jenkins_monitor.pid")
LOG_FILE = os.path.expanduser("~/.jenkins_monitor.log")
CONFIG_FILE = os.path.expanduser("~/.jenkins_monitor_jobs.json")

# --- Daemon Process State ---
# (These are only used when the script is running as the daemon)
g_job_states: Dict[str, bool] = {}
g_threads: list[threading.Thread] = []
g_token: str = ""


# --- Core Helper Functions ---


def setup_logging():
    """Configure logging to file only"""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - %(message)s",
        handlers=[logging.FileHandler(LOG_FILE)],
    )
    return logging.getLogger(__name__)


logger = setup_logging()


def is_daemon_running():
    """Check if the daemon is running and return its PID, otherwise return False."""
    try:
        with open(PID_FILE) as f:
            pid = int(f.read().strip())
        os.kill(pid, 0)
        return pid
    except (FileNotFoundError, ProcessLookupError, ValueError):
        # Clean up stale PID file if it exists
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
        return False


def send_macos_notification(title, message, url=""):
    """Send macOS system notification"""
    url_cmd = f"-open {url}" if url else ""
    cmd = f"terminal-notifier -message '{message}' -title '{title}' -sound ping {url_cmd} -group 'jenkins_monitor'"
    result = subprocess.run(
        cmd, shell=True, check=False, capture_output=True, text=True
    )
    if result.returncode != 0:
        logger.error(
            f"Failed to send notification: {cmd}. Stderr: {result.stderr.strip()}"
        )


def load_config() -> Set[str]:
    """Load job URLs from config file"""
    try:
        with open(CONFIG_FILE) as f:
            return set(json.load(f).get("jobs", []))
    except (FileNotFoundError, json.JSONDecodeError):
        return set()


def save_config(jobs: Set[str]):
    """Save job URLs to config file"""
    with open(CONFIG_FILE, "w") as f:
        json.dump({"jobs": list(jobs)}, f, indent=2)


# --- Jenkins API & Job Monitoring ---


def get_job_status(jenkins_url, token):
    """
    Get job status from Jenkins API.
    Returns (is_building, result) or a special status string on error.
    """
    api_url = jenkins_url.rstrip("/") + "/api/json?tree=building,result"
    headers = {"Authorization": f"Basic {token}"}
    try:
        response = requests.get(api_url, headers=headers, timeout=15)
        response.raise_for_status()
        data = response.json()
        return data.get("building"), data.get("result")
    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 404:
            logger.warning(f"Job not found (404) at {jenkins_url}")
            return "NOT_FOUND", None
        logger.error(f"HTTP error for {jenkins_url}: {e}")
    except requests.RequestException as e:
        logger.error(f"Connection error for {jenkins_url}: {e}")
    return None, None


def monitor_job(job_url: str):
    """Monitor a single job in a thread. Uses global state."""
    job_name = job_url.split("/job/")[-1].rstrip("/")
    logger.info(f"Started monitoring: {job_name}")

    last_result = None
    stop_reason = None

    while g_job_states.get(job_url, True):
        building, result = get_job_status(job_url, g_token)

        if building == "NOT_FOUND":
            logger.error(f"Job '{job_name}' returned 404. Removing.")
            send_macos_notification(
                "Jenkins Job Not Found",
                f"Job: {job_name}\nURL returned 404. Removing from monitor.",
                job_url,
            )
            stop_reason = "404 Not Found"
            break

        if building is None:
            logger.warning(f"Could not get status for {job_name}. Retrying in 30s.")
            time.sleep(30)
            continue

        if not building and result != last_result:
            status = result if result else "UNKNOWN"
            logger.info(f"Build finished: {job_name} - Status: {status}")
            send_macos_notification(
                "Jenkins Job Completed",
                f"Job: {job_name}\nStatus: {status}",
                job_url,
            )
            stop_reason = "build completed"
            break

        time.sleep(10)

    if stop_reason:
        g_job_states[job_url] = False
        jobs = load_config()
        jobs.discard(job_url)
        save_config(jobs)
        logger.info(f"Removed '{job_name}' from monitoring ({stop_reason}).")

    logger.info(f"Stopped monitoring: {job_name}")


# --- Daemon Control Functions ---


def _reload_daemon_config(is_initial_load=False):
    """Scan config and adjust monitoring threads. Uses global state."""
    global g_threads
    if not is_initial_load:
        logger.info("Reloading configuration...")

    new_jobs = load_config()
    active_jobs = {job for job, active in g_job_states.items() if active}

    # Stop threads for jobs that have been removed
    for job_url in active_jobs - new_jobs:
        logger.info(f"Stopping monitoring for removed job: {job_url}")
        g_job_states[job_url] = False

    # Start threads for new jobs
    for job_url in new_jobs - active_jobs:
        logger.info(f"Starting to monitor new job: {job_url}")
        g_job_states[job_url] = True
        thread = threading.Thread(target=monitor_job, args=(job_url,), daemon=True)
        thread.start()
        g_threads.append(thread)

    g_threads = [t for t in g_threads if t.is_alive()]
    if not is_initial_load:
        logger.info(f"Configuration reloaded. Monitoring {len(new_jobs)} jobs.")


def _sighup_handler(sig, frame):
    _reload_daemon_config()


def _shutdown_handler(sig, frame):
    """Gracefully shut down the daemon."""
    logger.info("Shutdown signal received, stopping all monitors.")
    for job_url in list(g_job_states.keys()):
        g_job_states[job_url] = False
    for thread in g_threads:
        thread.join(timeout=2)
    if os.path.exists(PID_FILE):
        os.remove(PID_FILE)
    logger.info("Daemon stopped.")
    sys.exit(0)


def start_daemon():
    """Start the daemon process."""
    pid = is_daemon_running()
    if pid:
        logger.error(f"Daemon already running (PID: {pid}).")
        return

    global g_token
    g_token = os.environ.get("JENKINS_TOKEN")
    if not g_token:
        logger.error("JENKINS_TOKEN environment variable not set!")
        return

    # Register signal handlers
    signal.signal(signal.SIGTERM, _shutdown_handler)
    signal.signal(signal.SIGINT, _shutdown_handler)
    signal.signal(signal.SIGHUP, _sighup_handler)

    with open(PID_FILE, "w") as f:
        f.write(str(os.getpid()))

    logger.info(f"Jenkins monitor daemon started (PID: {os.getpid()})")
    _reload_daemon_config(is_initial_load=True)
    logger.info(f"Monitoring {len(load_config())} job(s).")

    try:
        while True:
            if not load_config():
                logger.info("No more jobs to monitor. Shutting down daemon.")
                _shutdown_handler(None, None)
            time.sleep(5)
    except KeyboardInterrupt:
        _shutdown_handler(None, None)


# --- CLI Command Implementations ---


def _signal_daemon_reload():
    """Signal daemon to reload config. Returns True if daemon was signaled."""
    pid = is_daemon_running()
    if pid:
        os.kill(pid, signal.SIGHUP)
        return True
    return False


def _ensure_daemon_running():
    """Start daemon if not running. Prints status messages."""
    if is_daemon_running():
        return
    print("Daemon not running. Starting background monitor...")
    subprocess.Popen(
        [sys.executable, __file__, "_start_daemon"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        close_fds=True,
    )
    time.sleep(1)
    if is_daemon_running():
        print(f"{GREEN}Daemon started successfully.{ENDC}")
    else:
        print(f"{RED}Failed to start daemon. Check logs at {LOG_FILE}{ENDC}")


def add_job(job_url: str):
    """Add a job to monitor. Starts daemon if not running."""
    jobs = load_config()
    if job_url in jobs:
        print(f"{YELLOW}Job is already being monitored: {job_url}{ENDC}")
        return

    jobs.add(job_url)
    save_config(jobs)
    print(f"{GREEN}Added job to config: {job_url}{ENDC}")
    logger.info(f"Job added to config: {job_url}")

    if _signal_daemon_reload():
        print("Daemon signaled to monitor the new job.")
    else:
        _ensure_daemon_running()


def remove_job(job_url: str):
    """Remove a job from monitoring."""
    jobs = load_config()
    if job_url not in jobs:
        print(f"{YELLOW}Job not found in config: {job_url}{ENDC}")
        return

    jobs.discard(job_url)
    save_config(jobs)
    print(f"{GREEN}Removed job from config: {job_url}{ENDC}")
    logger.info(f"Job removed from config: {job_url}")

    if _signal_daemon_reload():
        print("Daemon signaled to stop monitoring the job.")


def stop_daemon():
    """Stop the daemon."""
    pid = is_daemon_running()
    if not pid:
        print(f"{YELLOW}Daemon not running.{ENDC}")
        return

    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        print(f"{GREEN}Daemon already stopped.{ENDC}")
        return

    logger.info(f"Sent SIGTERM to daemon (PID: {pid})")
    time.sleep(1)
    if is_daemon_running():
        print(f"{YELLOW}Daemon may still be shutting down.{ENDC}")
    else:
        print(f"{GREEN}Daemon stopped successfully.{ENDC}")


def get_status():
    """Get daemon status and monitored jobs."""
    pid = is_daemon_running()
    if not pid:
        print(f"{RED}Daemon not running.{ENDC}")
        return

    print(f"{GREEN}Daemon running (PID: {pid}){ENDC}")
    jobs = load_config()
    if jobs:
        print(f"Monitoring {len(jobs)} job(s):")
        for job in sorted(list(jobs)):
            print(f"  - {job}")
    else:
        print("Not monitoring any jobs.")


def tail_logs():
    """Follow (tail) the log file."""
    if not os.path.exists(LOG_FILE):
        print(f"Log file not found: {LOG_FILE}")
        return
    try:
        subprocess.run(["tail", "-f", LOG_FILE])
    except KeyboardInterrupt:
        print("\nStopped following logs.")
    except FileNotFoundError:
        print("Error: 'tail' command not found.")


def main():
    """Main CLI entrypoint."""
    if len(sys.argv) < 2 or sys.argv[1] in ["-h", "--help"]:
        print(f"Usage: {sys.argv[0]} {{add|remove|stop|status|logs}} [job_url]")
        sys.exit(1)

    commands = {
        "_start_daemon": start_daemon,
        "stop": stop_daemon,
        "status": get_status,
        "logs": tail_logs,
    }

    command = sys.argv[1]

    if command in commands:
        commands[command]()
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
    main()

#!/usr/bin/python3

import signal
import time

if __name__ == "__main__":
    is_running = True

    def shutdown(*args):
        global is_running
        is_running = False

    for sig in (signal.SIGTERM, signal.SIGINT):
        signal.signal(sig, shutdown)

    while is_running:
        time.sleep(0.5)

import subprocess
import threading

import yaml


class Worker(threading.Thread):
    def run(self):
        yaml.safe_load("a: 1")


if __name__ == "__main__":
    Worker().start()
    subprocess.run(["ls"])

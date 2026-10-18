import subprocess
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

import yaml


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        subprocess.run(["ls"])


class Worker(threading.Thread):
    def run(self):
        yaml.safe_load("a: 1")


if __name__ == "__main__":
    Worker().start()
    HTTPServer(("", 8000), Handler).serve_forever()

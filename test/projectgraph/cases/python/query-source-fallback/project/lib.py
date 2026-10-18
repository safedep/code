import json

import yaml


def run(path):
    with open(path) as f:
        return json.load(f)


def load_old(path):
    with open(path) as f:
        return yaml.load(f)

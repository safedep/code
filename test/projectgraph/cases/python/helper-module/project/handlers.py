"""Handlers of the configuration file."""

import yaml


def read(path):
    with open(path) as f:
        return f.read()


def parse(path):
    return yaml.load(read(path), Loader=yaml.SafeLoader)

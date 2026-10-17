import yaml


def parse(text):
    return yaml.safe_load(text)

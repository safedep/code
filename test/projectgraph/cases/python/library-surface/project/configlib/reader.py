import yaml


def read_config(text):
    return _parse(text)


def _parse(text):
    return yaml.safe_load(text)

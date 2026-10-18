import yaml


def create_app():
    yaml.safe_load(open("config.yaml"))
    return object()

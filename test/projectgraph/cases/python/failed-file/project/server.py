import yaml


def serve(text):
    """Serve the configuration. The padding makes the file larger than the limit of the case."""
    return yaml.load(text)

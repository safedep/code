import yaml


def run(cb):
    return cb("a: 1")


def process(text):
    return yaml.load(text)

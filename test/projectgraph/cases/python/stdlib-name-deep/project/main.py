import types

import yaml

from app.models.types import Record


def run():
    ns = types.SimpleNamespace(a=1)
    return yaml.load(Record(ns).text)


if __name__ == "__main__":
    run()

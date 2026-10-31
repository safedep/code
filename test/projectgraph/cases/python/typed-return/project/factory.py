from typing import Optional

import yaml


class Parser:
    def parse(self, text):
        return yaml.load(text)


def make() -> Parser:
    return Parser()


def maybe() -> Optional[Parser]:
    return None


def pair() -> tuple[Parser, int]:
    return Parser(), 1

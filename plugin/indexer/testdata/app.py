import os
import os.path
import json as j
from yaml import load
from yaml.loader import SafeLoader as Loader
from . import sibling
from .utils import helper
from ..pkg import thing as other
from collections import *


class Parser(Base, mixins.Mixin):
    def __init__(self, text):
        self.text = text

    def parse(self):
        return helper(self.text)

    def render(self):
        return self.parse()


def main():
    def nested():
        return os.path.join("a", "b")

    nested()
    p = Parser("x")
    p.render()
    load("a: 1", Loader=Loader)
    later()
    missing()
    attr = "parse"
    getattr(p, attr)()
    j.dumps(sibling, other)


def later():
    pass


main()

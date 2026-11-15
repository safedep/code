import functools
import os
import shutil
import subprocess
import tempfile
from dataclasses import dataclass, field


class ByClassmethod:
    def __call__(self):
        os.remove("a")

    @classmethod
    def create(cls):
        return cls()


class ByPartial:
    def __call__(self):
        shutil.rmtree("b")


class ByDefault:
    def __getattr__(self, name):
        subprocess.run("c")


@dataclass
class Holder:
    d: ByDefault = field(default_factory=ByDefault)


class Meta(type):
    def __call__(cls, *args):
        tempfile.mkdtemp()


def run(f):
    f()


def check(x):
    return isinstance(x, Meta)


if __name__ == "__main__":
    run(ByClassmethod.create())
    run(functools.partial(ByPartial)())
    print(Holder().d.anything)
    check(1)

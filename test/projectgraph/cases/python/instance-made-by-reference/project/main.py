import functools
import os
import shutil
import subprocess
import tempfile
import typing
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


class ByKlass:
    def __call__(self):
        os.chmod("e", 0)

    @classmethod
    def build(klass):
        def inner():
            return klass()

        return inner()


class ByInstanceClass:
    def __getattr__(self, name):
        os.rename("f", "g")

    def clone(self):
        return self.__class__()


class ByType:
    def __call__(self):
        os.rmdir("h")

    def dup(self):
        return type(self)()


class Raised(Exception):
    def __getattr__(self, name):
        os.unlink("i")


class Checked:
    def __call__(self):
        os.mkdir("j")


def run(f):
    f()


def fail():
    raise Raised


def copies(seed):
    seed.clone()
    seed.dup()


def check_more(x, c):
    typing.cast(Checked, x)
    hasattr(Checked, "x")
    return isinstance(x, Checked if c else Meta)


def check(x):
    return isinstance(x, Meta)


if __name__ == "__main__":
    run(ByClassmethod.create())
    run(functools.partial(ByPartial)())
    print(Holder().d.anything)
    check(1)
    run(ByKlass.build())
    copies(None)
    fail()
    check_more(1, True)

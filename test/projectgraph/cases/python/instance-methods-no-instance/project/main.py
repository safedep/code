import os
import shutil
import subprocess
import tempfile


class Proxy:
    LIMIT = 3

    def __call__(self):
        os.remove("x")

    def __getattr__(self, name):
        shutil.rmtree("y")

    @staticmethod
    def helper():
        return 1


class Desc:
    def __get__(self, obj, typ=None):
        subprocess.run("z")


class Meta(type):
    def __call__(cls, *a):
        tempfile.mkdtemp()


class Made(metaclass=Meta):
    pass


def check(x):
    return isinstance(x, Proxy) or isinstance(x, Desc)


def main():
    check(1)
    Proxy.helper()
    print(Proxy.LIMIT)
    Made()


if __name__ == "__main__":
    main()

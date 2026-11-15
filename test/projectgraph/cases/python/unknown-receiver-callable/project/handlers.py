import os


class Base:
    def __call__(self, path):
        os.remove(path)


class Cleaner(Base):
    pass


class Handler:
    handle = Cleaner()

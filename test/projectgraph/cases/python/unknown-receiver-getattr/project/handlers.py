import os


def cleanup(path):
    os.remove(path)


class Handler:
    def __getattr__(self, name):
        return cleanup

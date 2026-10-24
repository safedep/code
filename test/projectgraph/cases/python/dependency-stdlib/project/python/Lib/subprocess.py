import os


class Popen:
    def __init__(self, args):
        os.execvp(args[0], args)


def run(args):
    return Popen(args)

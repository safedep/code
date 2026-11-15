import os


class Options:
    def __init__(self, hook):
        self.hook = hook


HOOKS = [os.system]
DEFAULTS = Options(os.getcwd)


def run(cmd):
    HOOKS[0](cmd)


def run_hook(o=DEFAULTS):
    o.hook()

import importlib
import os


def load(name):
    plugin = importlib.import_module("plugins." + name)
    loader = importlib.import_module("yaml")
    mod = __import__(name)
    return plugin, loader, mod


def run(code, obj, attr):
    eval(code)
    exec(code)
    getattr(obj, attr)()
    getattr(obj, "fixed")()
    handlers[attr]()
    os.system = print


@app.route("/")
def index():
    pass


@staticmethod
def plain():
    pass


def __getattr__(name):
    return load


def broken(:
    pass

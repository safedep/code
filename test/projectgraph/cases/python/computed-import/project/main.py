import importlib
import sys


def load_plugin(name):
    module = importlib.import_module(name)
    return module.run()


if __name__ == "__main__":
    load_plugin(sys.argv[1])

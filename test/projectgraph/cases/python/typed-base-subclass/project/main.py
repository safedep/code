import dep
import yaml
from dep import Doer


class Mine(Doer):
    def do(self):
        yaml.load("a: 1")


def run(d: Doer):
    d.do()


def other(r: dep.Runner):
    r.run()


if __name__ == "__main__":
    run(Mine())
    other(dep.new_runner())

import abc
import subprocess


class Doer(abc.ABC):
    @abc.abstractmethod
    def do(self):
        raise NotImplementedError


class Runner(abc.ABC):
    @abc.abstractmethod
    def run(self):
        raise NotImplementedError


class Thing(Doer):
    def __init__(self):
        self.child = None

    def do(self):
        subprocess.run(["true"])

    def next(self) -> "Thing":
        return self.child


def new_thing() -> Thing:
    return Thing()


def new_pair() -> tuple[Thing, int]:
    return Thing(), 0


def try_thing() -> Thing | None:
    return Thing()


def new_doer() -> Doer:
    return Thing()


def new_runner() -> Runner:
    raise NotImplementedError

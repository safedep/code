from dataclasses import dataclass

from factory import Parser


@dataclass
class Job:
    parser: Parser

    def work(self):
        return self.parser.parse("a: 1")


class App:
    parser: Parser

    def __init__(self):
        self.parser = Parser()

    def go(self):
        return self.parser.parse("b: 2")


if __name__ == "__main__":
    Job(Parser()).work()
    App().go()

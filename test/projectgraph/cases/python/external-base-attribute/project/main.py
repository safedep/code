import threading

from parser import Parser


class Worker(threading.Thread):
    def __init__(self):
        super().__init__()
        self.parser = Parser()

    def work(self):
        return self.parser.parse("a: 1")


if __name__ == "__main__":
    Worker().work()

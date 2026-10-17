import threading

import yaml


class Base(threading.Thread):
    def run(self):
        yaml.safe_load("a: 1")


class Worker(Base):
    pass


class Loud(Base):
    def run(self):
        print("loud")
        super().run()


if __name__ == "__main__":
    Worker().start()

class Job:
    def __init__(self):
        self.cb = None

    def run(self):
        self.cb()


if __name__ == "__main__":
    Job().run()

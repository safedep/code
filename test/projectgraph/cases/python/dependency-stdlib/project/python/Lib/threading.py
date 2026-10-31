import _thread


class Thread:
    def start(self):
        _thread.start_new_thread(self._bootstrap, ())

    def _bootstrap(self):
        self.run()

    def run(self):
        pass

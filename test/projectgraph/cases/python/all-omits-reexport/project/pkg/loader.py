import yaml


class Loader:
    def load(self, text):
        return yaml.load(text)

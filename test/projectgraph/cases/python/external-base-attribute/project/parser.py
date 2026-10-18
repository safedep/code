import yaml


class Parser:
    def parse(self, text):
        return yaml.safe_load(text)

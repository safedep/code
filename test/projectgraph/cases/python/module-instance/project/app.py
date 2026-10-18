import yaml


class Parser:
    def parse(self, text):
        return yaml.load(text)


parser = Parser()

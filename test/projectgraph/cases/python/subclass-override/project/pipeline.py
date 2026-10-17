import yaml


class Step:
    def run(self, text):
        return self.parse(text)

    def parse(self, text):
        return text


class YamlStep(Step):
    def parse(self, text):
        return yaml.load(text)


class Mixin:
    def parse(self, text):
        return text


class Mixed(Mixin, YamlStep):
    pass

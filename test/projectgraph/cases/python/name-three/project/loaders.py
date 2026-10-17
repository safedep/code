import yaml


class YamlLoader:
    def load(self, text):
        return yaml.load(text)


class JsonLoader:
    def load(self, text):
        return text


def load(text):
    return text

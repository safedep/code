import yaml


class Base:
    def load(self, text):
        return yaml.load(text)


class Strict(Base):
    def load(self, text):
        return super().load(text)

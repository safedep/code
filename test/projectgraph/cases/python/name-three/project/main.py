from loaders import YamlLoader


def run(loader, text):
    return loader.load(text)


run(YamlLoader(), "a: 1")

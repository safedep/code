import yaml


def logged(fn):
    def wrapper(text):
        yaml.load(text)
        return fn(text)

    return wrapper


@logged
def handle(text):
    return text

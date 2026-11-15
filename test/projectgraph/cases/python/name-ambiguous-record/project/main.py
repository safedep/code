import yaml
from handlers import H0


def dispatch(handler, text):
    return handler.handle(text)


dispatch(H0(yaml), "a: 1")

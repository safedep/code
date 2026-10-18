import parsers


def dispatch(name, text):
    return getattr(parsers, name)(text)


if __name__ == "__main__":
    dispatch("parse", "a: 1")

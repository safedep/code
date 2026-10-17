import yaml


def dispatch(name, text):
    return getattr(yaml, name)(text)


if __name__ == "__main__":
    dispatch("safe_load", "a: 1")

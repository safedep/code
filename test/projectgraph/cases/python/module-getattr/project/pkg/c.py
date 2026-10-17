import marshal


def later(data):
    return marshal.loads(data)


def __getattr__(name):
    return later

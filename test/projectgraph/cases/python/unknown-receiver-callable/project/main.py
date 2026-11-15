from handlers import Handler


def dispatch(handler, path):
    return handler.handle(path)


dispatch(Handler(), "a")

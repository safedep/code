import plug

def dispatch(handler, text):
    return handler.handle(text)

dispatch(plug, 'a')

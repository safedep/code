import extlib
import handlers

def dispatch(handler, text):
    return handler.handle(text)

dispatch(extlib.Handler(), 'a')

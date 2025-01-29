import base64
from utils import printinit, printenc, printdec as pdec
    
class Encoding:
    def __init__(self):
        printinit("Initialized")
        pass

    def apply(self, msg, func):
        return func(msg)
    
    # Unused
    def apply2(self, msg, func):
        return func(msg)

def getenc():
    return "encoded"

encoder = Encoding()
encoded = encoder.apply("Hello, World!".encode('utf-8'), base64.b64encode)
printenc(encoded)
decoded = encoder.apply(getenc(), base64.b64decode)
pdec(decoded)

class NoConstructorClass:
    def show():
        print("NoConstructorClass")
        pass
ncc = NoConstructorClass()
ncc.show()
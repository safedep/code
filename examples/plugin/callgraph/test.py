import base64
from utils import printinit, printenc, printdec, printf2

# Node must be generated, but shouldn't be part of DFS
class EncodingUnused:
    def __init__(self):
        printinit("Initialized unused")
        pass

    def applyUnused(self, msg, func):
        return func(msg)
    
class Encoding:
    def __init__(self):
        printinit("Initialized")
        pass

    def apply(self, msg, func):
        return func(msg)
    
    # Unused
    def apply2(self, msg, func):
        return func(msg)

encoder = Encoding()
encoded = encoder.apply("Hello, World!".encode('utf-8'), base64.b64encode)
printenc(encoded)
decoded = encoder.apply(encoded, base64.b64decode)
printdec(decoded)


def f1(value):
  f2(value)

def f2(value):
  printf2(value)
  if value == 0:
    return
  f1(value-1)
  pass

def multiply(a, b):
    return a * b

f1(multiply(2, 3))

def foo():
  print("foo")
  pass

def bar():
  print("bar")
  pass

def baz():
  print("baz")
  pass
def useless():
  print("useless")
  baz()
  pass

xyz = foo

print("GG")

xyz = bar

xyz()
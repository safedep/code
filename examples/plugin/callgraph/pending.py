# Pending ---------------------------------------------

import base64
from utils import printinit, printenc, printdec, printf2

class SomeClass:
    def __init__(self):
        printinit("Initialized")
        pass
    def outer_method(self):
        print("Called outer_method")
        return self

# @TODO - Refer attributeResolver for more details
deepresultvalue = SomeClass().outer_method().inner_method()

# @TODO - This would require return value processing, which is a complex task
deepresultvalue.deepest_method()

# @TODO - We're not able to identify instance as return values from factory functions yet
def create_outer():
    return SomeClass()

# @TODO - Can't work with return values yet
a = SomeClass()
b = a.outer_method() # @TODO - class information needed for this
 


class Encoding:
    def __init__(self):
        pass
    def apply(self, msg, func):
        return func(msg)

encoder = Encoding()
encoded = encoder.apply("Hello, World!".encode('utf-8'), base64.b64encode)
printenc(encoded)
decoded = encoder.apply(encoded, base64.b64decode)
printdec(decoded)


# @TODO - Unable to resolve declaration afterwards (Python, Javascript, Go, etc support this feature)
def declaredFirst(value):
  declaredLater(value)
def declaredLater(value):
  print("GG", value)
declaredFirst(1)

# Another sample -
def f1(value):
  f2(value)
  pass
def f2(value):
  if value == 0:
    return
  f1(value-1)
f1(5)


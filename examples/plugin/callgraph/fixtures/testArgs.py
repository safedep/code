import pprint
from xyz import printxyz1, printxyz2, printxyz3
from os import getenv, EX_SOFTWARE 

def add(a, b):
    return a + b
def concat(a, b):
    return str(a) + str(b)
def multiply(a, b):
    return a * b

print(
  "gg", 1, 2.5, True, None, EX_SOFTWARE,

  [1, 2, 3], {"key": "value"}, (4, 5, multiply(2, 3)),
  
  add(5,3), concat("Hello, ", "World!"), add(7, add(8,9)), getenv("SOME_ENV_VAR"),
)


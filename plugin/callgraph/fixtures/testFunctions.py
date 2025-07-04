import pstats
import pprint
import random
from xyzprintmodule import printer1, printer2, printer3, printer4, printer6
from os import listdir as listdirfn, chmod

from os import getenv
from mypkg import SOME_CONSTANT 

# Recursive
def factorial(x):
   if x == 0 or x == 1:
       return 1
   else:
       return x * factorial(x-1)
print(factorial(5))


# Function assignment
def foo():
  pprint.pprint("foo")
def bar():
  print("bar")
baz = bar

xyz = "abc"
xyz = 25
xyz = foo
xyz = baz
xyz()


# Nested & scoped functions
def outerfn1():
  chmod("outerfn1")
  pass
def outerfn2():
  listdirfn("outerfn2")
  pass

def fn1():
  printer4("outer fn1")

def nestParent():
  def parentScopedFn():
    print("parentScopedFn")
    fn1() # Must call outer fn1 with printer4

  def nestChild():
    printer1("nestChild")
    outerfn1()
    
    def fn1():
      printer6("inner fn1")

    def childScopedFn():
      printer2("childScopedFn")
      fn1() # Must call outer fn1 with printer6

    def nestGrandChildUseless():
      printer3("nestGrandChildUseless")

    def nestGrandChild():
      pprint.pp("nestGrandChild")
      parentScopedFn()
      outerfn2()
      childScopedFn()

    nestGrandChild()

  outerfn1()
  nestChild()

nestParent()



# Assignments, return values aren't consumed, since it's a complex task
def add(a, b):
    return a + b
def sub(a, b):
    return a - b
def multiply(a, b):
    return a * b
somenumber = 5
r1 = 95 + 7.3 + 2

p1 = 599
if random.randint(0,1) == 0:
  p1 = "going good"
else:
  p1 = 39.2

p2 = 95
p2 = True

p3 = "gg"

res = add(p1, p2) + sub(p3, 6) + r1 - somenumber + 95 + 7.3 + pstats.getsomestat()


mul = multiply

def addProxy(a, b):
  return add(a, b)
def concat(a, b):
  return str(a) + str(b)

print(
  "gg", 1, 2.5, True, None, SOME_CONSTANT,

  [1, 2, 3], {"key": "value"}, (4, 5, mul(2, 3)),
  
  addProxy(5,3), concat("Hello, ", "World!"), add(p2, add(p1, p3)), getenv("SOME_ENV_VAR"),
)


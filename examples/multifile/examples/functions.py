import pstats
import pprint
from xyzprintmodule import printer1, printer2, printer3, printer4, printer6
from os import listdir as listdirfn, chmod

def fn1():
  printer4("outer fn1")

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




# Function Assignments, return values aren't processed, since its a complex taxk
def add(a, b):
    return a + b
def sub(a, b):
    return a - b
somenumber = 5
r1 = 95 + 7.3 + 2
res = add(3, 4) + sub(8, 6) + r1 - somenumber + 95 + 7.3 + pstats.getsomestat()

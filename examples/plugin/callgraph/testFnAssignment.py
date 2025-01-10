import pprint
from xyzprintmodule import xyzprint, xyzprint2, xyzprint3 as pxyz3

customprintxyz = pxyz3
customprintxyz = xyzprint2
customprintxyz("GG")


def foo():
  pprint.pprint("foo")
  pass

def bar():
  xyzprint("bar")
  pass

# unused
def baz():
  xyzprint2("baz")
  pass

xyz = foo
print("GG")

xyz = bar

xyz() # current analysis will simulate both foo() & bar() calls

def nestParent():
  def nestChild():
    xyzprint("nestChild")
    def nestGrandChild():
      xyzprint2("nestGrandChild")
      pass
    nestGrandChild()
  nestChild()
nestParent()

def useless():
  print("useless")
  baz()
  pass

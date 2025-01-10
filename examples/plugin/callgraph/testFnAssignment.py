import pprint
from xyzprintmodule import xyzprint, xyzprint2

def foo():
  pprint.pprint("foo")
  pass

def bar():
  xyzprint("bar")
  pass

def baz():
  xyzprint2("baz")
  pass

xyz = foo
print("GG")

xyz = bar
xyz()

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

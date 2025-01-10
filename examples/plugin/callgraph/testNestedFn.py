import pprint
from xyzprintmodule import xyzprint, xyzprint2
from os import listdir as listdirfn, chmod

def outerfn1():
  chmod("outerfn1")
  pass
def outerfn2():
  listdirfn("outerfn2")
  pass

def nestParent():
  def parentScopedFn():
    xyzprint("parentScopedFn")

  def nestChild():
    xyzprint("nestChild")
    outerfn1()

    def childScopedFn():
      xyzprint("childScopedFn")

    def nestGrandChildUseless():
      xyzprint2("nestGrandChildUseless")

    def nestGrandChild():
      pprint.pp("nestGrandChild")
      parentScopedFn()
      outerfn2()
      childScopedFn()

    nestGrandChild()

  outerfn1()
  nestChild()

nestParent()


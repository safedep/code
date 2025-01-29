import pprint
from xyz import printxyz as pxyz, printxyz2, printxyz3
from os import listdir as listdirfn, chmod

class ClassA:
  def __init__(self):
    pxyz("init")

  def method1(self):
    printxyz2("GG")


class ClassB:
  def __init__(self):
    pxyz("init")

  def method1(self):
    printxyz2("GG")

  def methodUnique(self):
    printxyz3("GG")
    pprint.pp("GG")


def main():
  x = ClassA()
  x = ClassB()
  y = x
  y.method1()
  y.methodUnique()
main()
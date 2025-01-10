from xyz import printxyz as pxyz, printxyz2

class ClassA:
  def __init__(self):
    pxyz("init")

  def method1(self):
    printxyz2("GG")

def main():
  x = ClassA()
  y = x
  y.method1()
main()
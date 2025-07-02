import pprint
from xyz import printxyz1, printxyz2, printxyz3
from os import getenv

# Correctly processes constructor, member function and member variables by instance keyword ie. self.name, self.value
class TesterClass:
    def __init__(self):
        self.name = "TesterClass name"
        self.value = 42
        if getenv("USE_TAR"):
            self.value = 100
    
    def helper_method(self):
        print("Called helper_method")
        return self.value
    
    def deepest_method(self):
        self.helper_method()
        print("Called deepest_method")
        return "Success"

    def aboutme(self):
        print(f"Name: {self.name}")
    
# Correctly identifies that alice is an instance of TesterClass
# so any qualifier on alice is resolved as a member of TesterClass
alice = TesterClass()
alice.aboutme()
bannername = alice.name




class ClassA:
  def method1(self):
    printxyz2("GG")
  def method2(self):
    printxyz2("GG")

class ClassB:
  def method1(self):
    printxyz2("GG")
  def method2(self):
    printxyz2("GG")
  def methodUnique(self):
    printxyz3("GG")
    pprint.pp("GG")


x = ClassA()
x = ClassB()
x.method1() 
y = x
y.method1()
y.method2()
y.methodUnique() # @TODO - This creates a call to namespace that doesn't exist



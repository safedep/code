import requests
from os import path

somenum = 5
someothernum = 7.0
dsf = "dsfa"
sf = 'f'
#
def add(a, b):
    return a + b
def sub(a, b):
    return a - b
def complexop(a, b):
    def add(a,b):
        return a*2 + b*2
    x = a
    return add(x, b) + add(a*2, b) + sub(a*2, b)
#
#
r1 = 95 + 7.3 + 2

# @TODO - handle assignment of return value of add/complex to res
res = complexop(1, 2) + add(3, 4) + add(5, 6) + somenum - someothernum + 95 + 7.3

print(r1)

xyz = "something"
pqr = "misc"
pqr = xyz
abc = pqr

class DeepestClass:
    def __init__(self):
        self.name = "DeepestClass"
    
    def deepest_method(self):
        print("Called deepest_method")
        return "Success"

# modelname = "gpt-3.5-turbo"
# Openai(modelname)

class InnerClass:
    def __init__(self):
        self.name = "InnerClass"
    
    def inner_method(self):
        print("Called inner_method")
        return DeepestClass()

class OuterClass:
    def __init__(self):
        self.name = "OuterClass"
    
    def outer_method(self):
        print("Called outer_method")
        return InnerClass()

def create_outer():
    return OuterClass()

def print_info(message):
    print(f"Info: {message}")

import requests
import requests
import requests
import requests
import requests


result = OuterClass().outer_method().inner_method()
    
# Use the assigned result
result.deepest_method()

# Multiple variables referencing the same chain
a = OuterClass()
b = a.outer_method()

# 
c = b.inner_method()
c.deepest_method()

# Testing different levels of attribute chains
def test_nested_attributes():
    # Level 1 attribute access
    outer = OuterClass()
    outer.outer_method()
    
    # Level 2 attribute access
    outer_inner = outer.outer_method()
    outer_inner.inner_method()
    
    # Level 3 attribute access
    outer_inner_deepest = outer.outer_method().inner_method()
    outer_inner_deepest.deepest_method()
    
    # Complex chaining in one line
    outer.outer_method().inner_method().deepest_method()
    
    # Create via helper function and chain
    create_outer().outer_method().inner_method()
    
    # External module attribute chaining
    requests.get("https://example.com").json()
    
    # Python standard library chaining
    path.dirname(path.abspath(__file__))

# Test variable assignment with attribute chains
def test_variable_assignments():
    # Assign object with multiple attribute levels
    result = OuterClass().outer_method().inner_method()
    
    # Use the assigned result
    result.deepest_method()
    
    # Multiple variables referencing the same chain
    a = OuterClass()
    b = a.outer_method()
    c = b.inner_method()
    c.deepest_method()

if __name__ == "__main__":
    test_nested_attributes()
    test_variable_assignments()
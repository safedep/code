"doc string here"
# single line comment
somevar = '''this is some correct value''' # accompanied with a comment
print(somevar) # this is also a comment

'''
Docstring outside func
'''
def func(arg: str):
    '''
    Docstring inside func
    '''
    print("func-", arg) ###### comment here
    pass

# print("Commented out code")
print("log1")#
func("some string don't mistake it for comment - # this is valid code")
func("""some str""")##

class MyClass:
    """
    Docstring inside class
    """
    def __init__(self, arg: str):
        print("MyClass-", arg)
        pass
    print("log7")
    pass
"""
All these are valid expressions
"""
g = MyClass("gggg")
g = MyClass('gggg')
g = MyClass('''gggg''')
g = MyClass("""gggg""")



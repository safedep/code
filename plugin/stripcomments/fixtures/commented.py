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
    def nested_func():
        """
        Docstring inside nested func
        """
        print("nested_func")#
        pass
    nested_func()
    pass

# print("Commented out code")
print("log1")#
func("don't mistake this string for comment - # this is valid code")
func("""some str""")##

class MyClass:
    """
    Docstring inside class
    """
    def __init__(self, arg: str):
        '''Docstring inside class function'''
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



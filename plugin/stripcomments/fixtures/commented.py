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
        if 4<9:
            if 5<3:
                print("deeper_nested_func")
                # single line comment
            else:
                print("deeper_nested_func")
                while(1):
                    print("deepest_nested_func") ###### comment here
                    if 5>3:
                        break
                match 'a':
                    case 'a':
                        if 3<10:
                            print("a_if")
                            '''further comment'''
                        print("a")
                    case 'b':
                        match "zs":
                            # single line comment
                            case "zs":
                                print("zs")
                            case _:
                                print("default")
                        print("b") # some comments
                    case _:
                        print("default")###
            print("deep_nested_func")
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



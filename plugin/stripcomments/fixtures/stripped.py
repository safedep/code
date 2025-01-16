

somevar = '''this is some correct value''' 
print(somevar) 


def func(arg: str):
    
    print("func-", arg) 
    def nested_func():
        
        print("nested_func")
        pass
    nested_func()
    pass


print("log1")
func("don't mistake this string for comment - # this is valid code")
func("""some str""")

class MyClass:
    
    def __init__(self, arg: str):
        
        print("MyClass-", arg)
        pass
    print("log7")
    pass


g = MyClass("gggg")
g = MyClass('gggg')
g = MyClass('''gggg''')
g = MyClass("""gggg""")
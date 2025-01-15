

somevar = '''this is some correct value''' 
print(somevar) 


def func(arg: str):
    
    print("func-", arg) 
    pass


print("log1")
func("some string don't mistake it for comment - # this is valid code")
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
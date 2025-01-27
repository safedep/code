

somevar = '''this is some correct value''' 
print(somevar) 


def func(arg: str):
    
    print("func-", arg) 
    def nested_func():
        
        if 4<9:
            if 5<3:
                print("deeper_nested_func")
                
            else:
                print("deeper_nested_func")
                while(1):
                    print("deepest_nested_func") 
                    if 5>3:
                        break
                match 'a':
                    case 'a':
                        if 3<10:
                            print("a_if")
                            
                        print("a")
                    case 'b':
                        match "zs":
                            
                            case "zs":
                                print("zs")
                            case _:
                                print("default")
                        print("b") 
                    case _:
                        print("default")
            print("deep_nested_func")
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
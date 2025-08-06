import asyncio

# A simple function
def simple_function():
    pass

# A function with arguments and type hints
def function_with_args(a: int, b: str) -> str:
    return f"{b}: {a}"

# An async function
async def my_async_function():
    await asyncio.sleep(1)
    return "done"

# A class with methods
class MyClass:
    def __init__(self, name):
        self.name = name

    def instance_method(self, value):
        return f"{self.name}: {value}"

    @staticmethod
    def static_method():
        return "static"

    @classmethod
    def class_method(cls):
        return "class_method"

# A decorated function
def my_decorator(func):
    def wrapper(*args, **kwargs):
        return func(*args, **kwargs)
    return wrapper

@my_decorator
def decorated_function():
    return "decorated"

# A nested function
def outer_function():
    def inner_function():
        pass
    return inner_function

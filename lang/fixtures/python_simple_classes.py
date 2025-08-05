# Simple Python classes for basic testing
# Tests fundamental class resolution without complex inheritance

class SimpleClass:
    def __init__(self):
        self.value = 42
    
    def get_value(self):
        return self.value

class ClassWithMethods:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name
    
    def set_name(self, new_name):
        self.name = new_name
    
    def process(self):
        return f"Processing {self.name}"

class ClassWithFields:
    class_var = "shared"
    
    def __init__(self):
        self.instance_var = "unique"
        self.counter = 0
    
    def increment(self):
        self.counter += 1

@property
class DecoratedClass:
    def __init__(self, value):
        self._value = value
    
    @property
    def value(self):
        return self._value
    
    @value.setter
    def value(self, new_value):
        self._value = new_value

# Class with no inheritance for baseline testing
class StandaloneClass:
    def standalone_method(self):
        return "standalone"
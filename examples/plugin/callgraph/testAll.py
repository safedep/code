import requests
import parser
import pstats
import zipfile
import tarfile
import gettext
import flask
from openai import Openai
from os import path, listdir, getenv, chdir


# Correct callgraph & assignment resolution ------------------------------------------

# Correctly assigned to appropriate imports
requests.get("https://example.com/" + chdir("something"))
Openai("gpt-3.5-turbo")

# Correctly assigned to builtin keyword - print
print("Hello")

# Archiver assignment to zipfile.ZipFile and tarfile.open.makearchive detected correctly
archiver = zipfile.ZipFile
if getenv("USE_TAR"):
    archiver = tarfile.open.makearchive

# Function Calls (path.join) added to call from current namespace (here filename) 
# Note - return values & arg assignments aren't processed
archiver(path.join("something", gettext.get("xyz")))

# Parsed correctly
path.altsep.capitalize(
    "something",
    getenv("xyz"),
    parser.parse("https://example.com")
)

# Literal assignment
somenumber = 7.0

# Correctly assigned multiple attribute values
abc = path.altsep.__dict__
abc = path.altsep
abc = listdir
abc = requests.__url__
abc = 7
abc = True
abc = "gg"
abc = somenumber

# This forms a chain of assignments
# spd => abc => [listdir, 7, True, somenumber, ....]
spd = abc

# Attribute assignee
path.altsep.__dict__ = "something"
path.altsep.__dict__ = "something else"


# Nested function definitions & scoped calls correctly parsed
def add(a, b):
    return a + b
def sub(a, b):
    return a - b
def complexop(a, b):
    def add(a,b):
        return a*2 + b*2
    x = a
    return add(x, b) + add(a*2, b) + sub(a*2, b)

r1 = 95 + 7.3 + 2
res = complexop(1, 2) + add(3, 4) + add(5, 6) + r1 - somenumber + 95 + 7.3 + pstats.getsomestat()

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
    
# Correctly identifies that adfff is instance of TesterClass
# so any qualifier on adfff is resolved as member of TesterClass
alice = TesterClass()
alice.aboutme()
bannername = alice.name

# 1. Importing an entire module
import prototurk
import plupload
# plupload.save("xyz", "abc")
import sys
print(sys.argv)

# 2. Importing a specific item from a module
from langchain_community import llms
from datetime import datetime
# print(datetime.now())
from math import sqrt
print(sqrt(16))

# 3. Importing a module/submodule/item with an alias
import plib as pl
import pandas as pd
df = pd.DataFrame({'A': [1, 2], 'B': [3, 4]})

import langchain.chat_models as customchat
import matplotlib.pyplot as plt
plt.plot([1, 2, 3], [1, 4, 9])

# 4. Importing with an alias for a specific function
from encodings import utf_8 as enc, ascii as asc, utf_8_sig as ut8 # none used
from slumber import API as sl, exceptions as ex # one is used
sl.get('https://example.com')
from sklearn import datasets as ds, metrics as met # all used
ds.load_iris()
score = met.accuracy_score([1, 2, 3], [1, 2, 3])

# 5. Importing multiple specific functions from a module
from odbc import connect, fetch # none is used 
from random import choices, randint, gauss # one of them is used
print(randint(1, 10))
from collections import deque, defaultdict, namedtuple # all are used
queue = deque([1, 2, 3])
queue.append(4)
dfd = defaultdict(int)
nmd = namedtuple('Person', ['name', 'age'])

# 6. Importing a function from a submodule
from oauthlib.oauth2 import WebApplicationClient
from json.decoder import JSONDecodeError
from urllib.parse import urlparse
print(urlparse('https://example.com'))

# 7. Importing a module conditionally
try:
    import ujson
    ujson.decode('{"a": 1, "b": 2}')
    import plistlib as plb
except ImportError:
    import simplejson as smpjson
    smpjson.dumps({'a': 1, 'b': 2})

# @TODO - How to resolve such usage ? -----------
# 8. Importing all functions from a module / submodule
from seaborn import * 
from flask.helpers import *

# @TODO - How to resolve such usage ? -----------
# 9. Importing a module/submodule for side effects only (no specific usage) 
import logging # although its not called by our code, it still initializes the logging module behind the scenes



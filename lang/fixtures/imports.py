# 1. Importing an entire module
import prototurk
import sys

# 2. Importing a module/submodule/item with an alias
import pandas as pd
import langchain.chat_models as customchat
import matplotlib.pyplot as plt

# 3. Importing a module conditionally
try:
    import ujson
    import plistlib as plb
except ImportError:
    import simplejson as smpjson

# 4. Importing all functions from a module / submodule
from seaborn import * 
from flask.helpers import *
from xyz.pqr.mno import *

# 5. Importing a specific item from a module
from math import sqrt
from langchain_community import llms

# 6. Importing with/without an alias for a specific function
from odbc import connect, fetch
from sklearn import datasets as ds, metric, preprocessing as pre
from oauthlib.oauth2 import WebApplicationClient as WAC, WebApplicationServer

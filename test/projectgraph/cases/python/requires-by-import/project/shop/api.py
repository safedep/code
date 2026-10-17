from flask import Flask

import pickle

app = Flask(__name__)


@app.route("/load")
def load():
    return pickle.loads(b"")

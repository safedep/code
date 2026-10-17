from flask import Flask
import yaml

api = Flask(__name__)


@api.route("/config")
def config():
    return yaml.safe_load("a: 1")


def helper():
    return yaml.load("b: 1")

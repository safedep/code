import os
import shutil

import jsonschema


def unused():
    os.remove("x")
    jsonschema.validate(1, {})


shutil.rmtree("y")

import os
import shutil


def unused():
    os.remove("x")


shutil.rmtree("y")

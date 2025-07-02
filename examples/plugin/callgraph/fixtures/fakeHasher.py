from random import random
import customlib.crypto.hasher as customhasher

def hash_string(input_string: str):
  hashedValue = customhasher(input_string, "md5")
  return hashedValue

def hash_with_random_algo(input_string: str):
  algo = "md5"
  if random.random() < 0.33:
    algo = "sha256"
  else:
    algo = "sha512"
  hashedValue = customhasher(input_string, algo)
  return hashedValue

hash_string("example input")

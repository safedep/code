from argon2 import PasswordHasher
import hashlib


def hash_password(p):
    PasswordHasher().hash(p)
    return hashlib.sha256(p).hexdigest()

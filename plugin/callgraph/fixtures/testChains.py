from argon2 import PasswordHasher
import hashlib
from openai import OpenAI


def hash_password(p):
    PasswordHasher().hash(p)
    return hashlib.sha256(p).hexdigest()


def ask(prompt):
    return OpenAI().chat.completions.create(model="gpt-4o", messages=[prompt])

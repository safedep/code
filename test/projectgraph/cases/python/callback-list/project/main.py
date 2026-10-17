import json

import yaml

HANDLERS = [json.loads, yaml.load]

for handler in HANDLERS:
    handler("{}")

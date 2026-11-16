import yaml


def handler(event, context):
    return yaml.load(event["body"])

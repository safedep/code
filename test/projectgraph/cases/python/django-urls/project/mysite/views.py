import yaml


def home(request):
    return yaml.safe_load("a: 1")


def health(request):
    return "ok"

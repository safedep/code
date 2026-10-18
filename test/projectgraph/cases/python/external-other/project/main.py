import requests
import yaml


def fetch(url):
    return requests.get(url).text


def dump(data):
    return yaml.dump(data)


if __name__ == "__main__":
    print(fetch("https://example.com"))

import yaml


def test_load():
    assert yaml.safe_load("a: 1") == {"a": 1}

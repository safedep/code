import yaml

handlers = {"log": print}
name = "log"


@handlers[name]
def run():
    yaml.load("a: 1")


if __name__ == "__main__":
    run()

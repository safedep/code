from factory import make


def run():
    p = make()
    p.parse("a: 1")


if __name__ == "__main__":
    run()

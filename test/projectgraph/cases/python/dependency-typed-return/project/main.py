import dep


def run():
    t = dep.new_thing()
    t.do()
    d = dep.new_doer()
    d.do()


if __name__ == "__main__":
    run()

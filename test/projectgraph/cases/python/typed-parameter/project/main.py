from typing import Optional

from factory import Parser


def run(p: Parser, o: Optional[Parser], u: Parser | None, raw):
    p.parse("a: 1")
    o.parse("b: 2")
    u.parse("c: 3")
    raw.parse("d: 4")


if __name__ == "__main__":
    run(Parser(), None, None, Parser())

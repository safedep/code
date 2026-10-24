from factory import make, maybe, pair

make().parse("a: 1")
p = make()
p.parse("b: 2")
m = maybe()
m.parse("c: 3")
q, n = pair()
q.parse("d: 4")

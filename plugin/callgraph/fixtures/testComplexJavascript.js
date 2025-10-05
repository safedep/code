// Complex JavaScript file to test infinite loop/high complexity fixes

// Deep member expression nesting (should trigger depth limit)
const deepNested = a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z;

// Polymorphic assignments (should trigger polymorphic limit)
let poly = new ClassA();
poly = new ClassB();
poly = new ClassC();
poly = new ClassD();
poly = new ClassE();
poly.method1();
poly.method2();

// Circular-like assignments
let x = y;
let y = z;
let z = w;
let w = x; // Creates potential cycle

// Many classes to create large assignment graph
class Class1 { method() {} }
class Class2 { method() {} }
class Class3 { method() {} }
class Class4 { method() {} }
class Class5 { method() {} }
class Class6 { method() {} }
class Class7 { method() {} }
class Class8 { method() {} }
class Class9 { method() {} }
class Class10 { method() {} }

// Chained method calls
obj.method1().method2().method3().method4().method5().method6().method7().method8();

// Deeply nested object access
const result = obj.prop1.prop2.prop3.prop4.prop5.prop6.prop7.prop8.prop9.prop10;

// Multiple reassignments
let variable = obj1;
variable = obj2;
variable = obj3;
variable = obj4;
variable = obj5;
variable.doSomething();

// Complex member expressions with calls
a.b.c().d.e().f.g().h.i().j.k().l.m().n.o().p.q();

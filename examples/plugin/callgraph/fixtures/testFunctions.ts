import * as pprint from 'pprint';
import { printer1, printer2, printer3, printer4, printer6 } from 'xyzprintmodule';
import { listdir as listdirfn, chmod } from 'os';

// Recursive
function factorial(x: number): number {
  if (x === 0 || x === 1) {
    return 1;
  } else {
    return x * factorial(x - 1);
  }
}
console.log(factorial(5));

// Function assignment
function foo(): void {
  pprint.pprint("foo");
}
function bar(): void {
  console.log("bar");
}
const baz = bar;

let xyz: any = "abc";
xyz = 25;
xyz = foo;
xyz = baz;
xyz();

// Nested & scoped functions
function outerfn1(): void {
  chmod("outerfn1");
}
function outerfn2(): void {
  listdirfn("outerfn2");
}

function fn1(): void {
  printer4("outer fn1");
}

function nestParent(): void {
  function parentScopedFn(): void {
    console.log("parentScopedFn");
    fn1(); // Must call outer fn1 with printer4
  }

  function nestChild(): void {
    printer1("nestChild");
    outerfn1();

    function fn1(): void {
      printer6("inner fn1");
    }

    function childScopedFn(): void {
      printer2("childScopedFn");
      fn1(); // Must call inner fn1 with printer6
    }

    function nestGrandChildUseless(): void {
      printer3("nestGrandChildUseless");
    }

    function nestGrandChild(): void {
      pprint.pp("nestGrandChild");
      parentScopedFn();
      outerfn2();
      childScopedFn();
    }

    nestGrandChild();
  }

  outerfn1();
  nestChild();
}

nestParent();

// Function assignments, return values aren't processed
function add(a: number, b: number): number {
  return a + b;
}
function sub(a: number, b: number): number {
  return a - b;
}
const somenumber = 5;
const r1 = 95 + 7.3 + 2;
const res = add(3, 4) + sub(8, 6) + r1 - somenumber + 95 + 7.3;

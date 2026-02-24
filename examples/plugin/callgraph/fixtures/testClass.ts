import * as pprint from 'pprint';
import { printxyz1, printxyz2, printxyz3 } from 'xyz';
import { getenv } from 'os';

// Correctly processes constructor, member function and member variables via this keyword
class TesterClass {
  name: string;
  value: number | string;

  constructor(newValue?: number) {
    this.name = "TesterClass name";
    this.value = 42;
    if (newValue !== undefined) {
      this.value = newValue;
    }
    if (getenv("USE_TAR")) {
      this.value = 100;
    } else {
      this.value = "default value";
    }
  }

  helperMethod(): number | string {
    console.log("Called helperMethod");
    return this.value;
  }

  deepestMethod(): string {
    this.helperMethod();
    console.log("Called deepestMethod");
    return "Success";
  }

  aboutme(): void {
    console.log(`Name: ${this.name}`);
  }
}

// Correctly identifies that alice is an instance of TesterClass
// so any qualifier on alice is resolved as a member of TesterClass
const alice = new TesterClass(35);
alice.aboutme();
const bannername = alice.name;

class ClassA {
  method1(): void {
    printxyz2("GG");
  }
  method2(): void {
    printxyz2("GG");
  }
}

class ClassB {
  method1(): void {
    printxyz2("GG");
  }
  method2(): void {
    printxyz2("GG");
  }
  methodUnique(): void {
    printxyz3("GG");
    pprint.pp("GG");
  }
}

let x: ClassA | ClassB = new ClassA();
x = new ClassB();
x.method1();
const y = x;
y.method1();
y.method2();
y.methodUnique();

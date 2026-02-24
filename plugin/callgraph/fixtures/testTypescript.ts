// Import statements
import axios from 'axios';
import { log, warn } from 'console';
import type { AxiosResponse } from 'axios';
import { createHash } from 'crypto';
import { printA1, printA2, printB1, printB2, printBU } from 'printers';

// Simple function declaration
function simpleFunction(param1: number, param2: number): number {
    log("Simple function called");
    return param1 + param2;
}

// Arrow function
const arrowFunc = (x: number): number => {
    warn("Arrow function called");
    return x * 2;
};

// Interface (should be skipped in callgraph)
interface Printable {
    print(): void;
}

// Class with constructor and methods
class TestClass implements Printable {
    private name: string;
    protected value: number;

    constructor(name: string, value: number) {
        this.name = name;
        this.value = value;
        log("TestClass constructor");
    }

    helperMethod(): number {
        log("Called helper method");
        return this.value;
    }

    deepMethod(): string {
        this.helperMethod();
        log("Called deep method");
        return "Success";
    }

    print(): void {
        log(this.name);
    }
}

// Create instance and call methods
const instance = new TestClass("test", 42);
instance.helperMethod();
instance.deepMethod();

// Module-level function calls
simpleFunction(1, 2);
arrowFunc(5);

// Additional module-level test
log("Module level call");

// Method calls on imported modules
axios.get("https://example.com");
const hash = createHash("sha256");

// Chained method calls
const result = instance.helperMethod().toString();

// Assignment from method call
const value = instance.helperMethod();

// Multiple class instances - use distinct callees per class so DFS can verify each
class ClassA {
    method1(): void {
        printA1("ClassA method1");
    }
    method2(): void {
        printA2("ClassA method2");
    }
}

class ClassB {
    method1(): void {
        printB1("ClassB method1");
    }
    method2(): void {
        printB2("ClassB method2");
    }
    methodUnique(): void {
        printBU("ClassB unique");
    }
}

// Polymorphic assignment
let x: ClassA | ClassB = new ClassA();
x = new ClassB();
x.method1();

const y = x;
y.method1();
y.method2();
y.methodUnique();

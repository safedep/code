// Import statements
const fs = require('fs');
const { readFile, writeFile } = require('fs/promises');
import axios from 'axios';
import { log, warn } from 'console';

// Simple function declaration
function simpleFunction(param1, param2) {
    log("Simple function called");
    return param1 + param2;
}

// Arrow function
const arrowFunc = (x) => {
    warn("Arrow function called");
    return x * 2;
};

// Class with constructor and methods
class TestClass {
    constructor(name, value) {
        this.name = name;
        this.value = value;
        log("TestClass constructor");
    }

    helperMethod() {
        log("Called helper method");
        return this.value;
    }

    deepMethod() {
        this.helperMethod();
        log("Called deep method");
        return "Success";
    }

    async asyncMethod() {
        const result = await readFile("test.txt");
        log("Async method");
        return result;
    }
}

// Create instance and call methods
const instance = new TestClass("test", 42);
instance.helperMethod();
instance.deepMethod();

// Module-level function calls - these should now be tracked!
simpleFunction(1, 2);
arrowFunc(5);

// Additional module-level test
log("Module level call");

// Method calls on imported modules
fs.readFileSync("file.txt");
axios.get("https://example.com");

// Chained method calls
const result = instance.helperMethod().toString();

// Assignment from method call
const name = instance.name;
const value = instance.helperMethod();

// Multiple class instances
class ClassA {
    method1() {
        log("ClassA method1");
    }
    method2() {
        warn("ClassA method2");
    }
}

class ClassB {
    method1() {
        log("ClassB method1");
    }
    method2() {
        warn("ClassB method2");
    }
    methodUnique() {
        log("ClassB unique");
    }
}

// Polymorphic assignment
let x = new ClassA();
x = new ClassB();
x.method1();

const y = x;
y.method1();
y.method2();
y.methodUnique();

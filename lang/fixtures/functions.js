// A simple function declaration
function declaredFunction(a, b) {
  return a + b;
}

// A function expression
const expressionFunction = function(x) {
  return x * x;
};

// An arrow function
const arrowFunction = (y) => {
  return y / 2;
};

// An async function
async function asyncFunction() {
  return Promise.resolve('done');
}

// A class with methods
class MyClass {
  constructor(name) {
    this.name = name;
  }

  myMethod(value) {
    return `${this.name}: ${value}`;
  }

  static staticMethod() {
    return "static";
  }

  get myProperty() {
    return this.name;
  }
}

// A decorated method (assuming decorators are enabled)
function myDecorator(target, key, descriptor) {
  // no-op
}

class ClassWithDecorator {
    @myDecorator
    decoratedMethod() {
        return 'decorated';
    }
}

// Regular function with type annotations
function declaredFunction(a: number, b: number): number {
  return a + b;
}

// Arrow function with types
const arrowFunction = (y: string): string => {
  return y.toUpperCase();
};

// Async function
async function asyncFunction(): Promise<string> {
  return Promise.resolve('done');
}

// Generic function
function identity<T>(arg: T): T {
  return arg;
}

// A class with methods
class MyClass {
  private name: string;

  constructor(name: string) {
    this.name = name;
  }

  public myMethod(value: number): string {
    return `${this.name}: ${value}`;
  }

  static staticMethod(): string {
    return "static";
  }

  get myProperty(): string {
    return this.name;
  }
}

// Abstract class with abstract method
abstract class AbstractService {
  abstract process(data: string): void;

  protected helper(): void {
    // shared logic
  }
}

// Decorated method
function myDecorator(target: any, key: string, descriptor: PropertyDescriptor) {
  // no-op
}

class ClassWithDecorator {
  @myDecorator
  decoratedMethod(): string {
    return 'decorated';
  }
}

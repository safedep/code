// Simple class
class SimpleClass {
  value: number;

  constructor(value: number) {
    this.value = value;
  }

  getValue(): number {
    return this.value;
  }
}

// Class with methods
class ClassWithMethods {
  greet(name: string): string {
    return `Hello, ${name}`;
  }

  calculate(a: number, b: number): number {
    return a + b;
  }
}

// Class with fields
class ClassWithFields {
  public name: string;
  private age: number;
  protected active: boolean;

  constructor(name: string, age: number) {
    this.name = name;
    this.age = age;
    this.active = true;
  }
}

// Interface
interface SimpleInterface {
  doSomething(): void;
  getValue(): string;
}

// Enum
enum Direction {
  Up = "UP",
  Down = "DOWN",
  Left = "LEFT",
  Right = "RIGHT",
}

// Decorated class
function Component(options: any) {
  return function(target: any) {};
}

@Component({ selector: 'app-root' })
class DecoratedClass {
  title = 'app';
}

// Standalone class
class StandaloneClass {
  run(): void {
    console.log("running");
  }
}

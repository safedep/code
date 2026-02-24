// Abstract base class
abstract class BaseService {
  protected config: Record<string, string>;

  constructor(config: Record<string, string>) {
    this.config = config;
  }

  abstract getServiceType(): string;

  getConfig(): Record<string, string> {
    return this.config;
  }

  isInitialized(): boolean {
    return this.config !== undefined;
  }
}

// Single inheritance
class StorageService extends BaseService {
  getServiceType(): string {
    return "storage";
  }

  store(key: string, value: string): void {
    this.config[key] = value;
  }
}

// Interfaces
interface Cacheable {
  cache(): void;
  invalidate(): void;
}

interface Loggable {
  log(message: string): void;
}

// Class extending class and implementing interfaces
class AdvancedStorageService extends StorageService implements Cacheable, Loggable {
  cache(): void {}
  invalidate(): void {}
  log(message: string): void {
    console.log(message);
  }
}

// Further single inheritance
class CloudStorageService extends AdvancedStorageService {
  upload(data: string): void {}
}

// Abstract class with decorator
@Deprecated
abstract class AbstractProcessor {
  abstract process(data: string): string;
  abstract validate(data: string): boolean;

  protected transform(data: string): string {
    return data.trim();
  }
}

function Deprecated(target: any) {}

// Single inheritance from abstract
class DataProcessor extends AbstractProcessor {
  process(data: string): string {
    return this.transform(data);
  }

  validate(data: string): boolean {
    return data.length > 0;
  }
}

// Deep inheritance chain
class Level1 extends BaseService {
  getServiceType(): string { return "level1"; }
}

class Level2 extends Level1 {
  getServiceType(): string { return "level2"; }
}

class Level3 extends Level2 {
  getServiceType(): string { return "level3"; }
}

class Level4 extends Level3 {
  getServiceType(): string { return "level4"; }
}

// Generic class with inheritance
class GenericService<T> extends BaseService {
  private data: T | null = null;

  getServiceType(): string { return "generic"; }

  setData(data: T): void {
    this.data = data;
  }
}

// Interface extending interface
interface ExtendedInterface extends Cacheable {
  refresh(): void;
}

// Nested/inner class pattern
class OuterClass {
  value: number = 0;

  getValue(): number {
    return this.value;
  }
}

// Test runner class (no inheritance)
class TestRunner {
  run(): void {}
}

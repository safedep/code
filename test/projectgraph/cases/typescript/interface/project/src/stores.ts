import yaml from 'js-yaml';
import type { Store } from './store';

export class YamlStore implements Store {
  get(key: string): unknown {
    return yaml.load(key);
  }
}

export class MemoryStore implements Store {
  get(key: string): unknown {
    return key;
  }
}

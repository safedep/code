import type { Store } from './store';
import { YamlStore } from './stores';

class App {
  constructor(private readonly store: Store) {}

  read(key: string): unknown {
    return this.store.get(key);
  }
}

new App(new YamlStore()).read('a: 1');

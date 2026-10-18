import { Core } from './core';
import { Impl } from './impl';

function handle(c: Core): void {
  c.run();
}

handle(new Impl());

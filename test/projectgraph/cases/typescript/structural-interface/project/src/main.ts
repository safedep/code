import type { Runner } from './runner';
import { YamlRunner } from './impl';
import { shell } from './plain';

function start(r: Runner): void {
  r.run('a: 1');
}

start(new YamlRunner());
start(shell);

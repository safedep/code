import { spawnSync } from 'child_process';

export default class {
  run() {
    spawnSync('make');
  }
}

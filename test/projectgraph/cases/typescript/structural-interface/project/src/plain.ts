import { execSync } from 'child_process';
import type { Runner } from './runner';

export const shell: Runner = {
  run(input: string): void {
    execSync(input);
  },
};

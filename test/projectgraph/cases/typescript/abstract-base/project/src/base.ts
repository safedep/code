import { execSync } from 'child_process';
import { Core } from './core';

export abstract class Base extends Core {
  run(): void {
    execSync('ls');
  }

  abstract name(): string;
}

import * as pprint from 'pprint';
import { printxyz1, printxyz2, printxyz3 } from 'xyz';
import { getenv, EX_SOFTWARE } from 'os';

function add(a: number, b: number): number {
  return a + b;
}
function concat(a: string, b: string): string {
  return a + b;
}
function multiply(a: number, b: number): number {
  return a * b;
}

console.log(
  "gg", 1, 2.5, true, null, EX_SOFTWARE,

  [1, 2, 3], { key: "value" }, [4, 5, multiply(2, 3)],

  add(5, 3), concat("Hello, ", "World!"), add(7, add(8, 9)), getenv("SOME_ENV_VAR"),
);

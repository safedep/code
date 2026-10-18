import { shared } from './shared';
import type { Options } from './types';

export function run(opts: Options): void {}
export const make = (): void => run({});
export abstract class Service extends Main {
  start(): void {}
}
export default class Main {}
function hidden(): void {}
export { hidden, shared as again };
export { parse as load } from './parser';
export * from './all';
export * as ns from './ns';
export interface Shape {}
export type Kind = string;

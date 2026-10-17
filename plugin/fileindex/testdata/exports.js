import { shared } from './shared';
import def from './def';

export function run() {}
export const make = () => run();
export class Service {
  start() {}
}
export default function main() {}
function hidden() {}
const local = 1;
export { hidden, local as renamed, shared, def as other };
export { parse as load } from './parser';
export * from './all';
export * as ns from './ns';
module.exports = hidden;
module.exports.extra = run;
exports.helper = shared;
exports.inline = function () {};

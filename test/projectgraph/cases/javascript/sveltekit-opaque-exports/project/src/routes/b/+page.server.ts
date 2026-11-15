import { a4, a5 } from 'plug';

function makeActions() {
  return { default: async () => a4() };
}

export const actions = makeActions();

const table = { save: async () => a5() };
const named = table;

export { named as other };

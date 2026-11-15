import { a2 } from 'plug';

function makeAll() {
  return { GET: async () => { a2(); } };
}

export const handlers = makeAll();

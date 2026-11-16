import { a5 } from 'plug';

function makeApi() {
  return { get() { a5(); } };
}

const api = makeApi();

export { api };

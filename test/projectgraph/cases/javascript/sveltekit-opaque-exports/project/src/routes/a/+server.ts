import { a1, a3 } from 'plug';

function make() {
  return { GET() { a1(); } };
}

export const { GET } = make();

const fallback = () => a3();
const handlers = {};

export const { PUT = fallback } = handlers;

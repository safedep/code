import { a1, a2, a3, a4 } from 'plug';

const handlers = {
  GET: async () => a1(),
  rest: { PUT: async () => a2() },
  post() {
    a3();
  },
};
const fallbacks = [async () => a4()];

export const { GET, rest: { PUT }, post: POST } = handlers;
export const [fallback] = fallbacks;

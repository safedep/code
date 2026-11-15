import { a1, a2 } from 'plug';

export const actions = {
  default: async ({ request }) => {
    a1(request);
  },
  save({ request }) {
    a2(request);
  },
};

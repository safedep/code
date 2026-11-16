import { a1, a2, a7 } from 'plug';

export const actions = {
  default: async ({ request }) => {
    a1(request);
  },
  save({ request }) {
    function unused() {
      a7();
    }
    a2(request);
  },
};

import { createHandler } from 'kit';
import { a3 } from 'plug';

type Handler = (req: Request) => Promise<Response>;

export const GET = <Handler>createHandler({
  onRequest() {
    a3();
  },
});

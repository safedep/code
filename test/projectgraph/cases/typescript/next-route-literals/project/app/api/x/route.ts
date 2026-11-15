import type { Metadata } from 'next';
import { a1 } from 'plug';

export const revalidate = 60 * 60;
export const dynamic = 'force-dynamic' as const;
export const maxDuration = -1;
export const metadata = { title: 'x' } satisfies Metadata;
export const config = { api: { bodyParser: !process.env.RAW } };

export async function GET() {
  a1();
}

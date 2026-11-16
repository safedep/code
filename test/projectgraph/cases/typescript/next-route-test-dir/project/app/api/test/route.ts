import { probe } from 'plug';

export async function GET() {
  return probe();
}

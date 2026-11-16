import { probe } from 'plug';

export function GET() {
  return probe();
}

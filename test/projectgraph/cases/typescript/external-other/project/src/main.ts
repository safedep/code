import axios from 'axios';
import yaml from 'js-yaml';

async function fetchText(url: string): Promise<string> {
  const response = await axios.get<string>(url);
  return response.data;
}

export function dump(value: unknown): string {
  return yaml.dump(value);
}

fetchText(process.argv[2]).then(console.log);

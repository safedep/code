import yaml from 'js-yaml';

export async function GET(): Promise<Response> {
  return Response.json(yaml.load('a: 1'));
}

function unused(): unknown {
  return yaml.dump({});
}

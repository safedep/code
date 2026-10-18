const axios = require('axios');
const yaml = require('js-yaml');

async function fetchText(url) {
  return (await axios.get(url)).data;
}

function dump(value) {
  return yaml.dump(value);
}

fetchText(process.argv[2]).then(console.log);

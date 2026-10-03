const crypto = require('crypto');
import OpenAI from 'openai';

const digest = crypto.createHash('md5').update('x').digest('hex');

async function ask() {
  return await new OpenAI().chat.completions.create({ model: 'gpt-4o' });
}

async function loadEmbedder() {
  return import("@xenova/transformers").then(({ pipeline }) => pipeline("feature-extraction"));
}

function client() {
  return OpenAI.createClient("k").chat.completions.create({});
}

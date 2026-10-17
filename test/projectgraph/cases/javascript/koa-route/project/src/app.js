const Koa = require('koa');
const Router = require('@koa/router');
const yaml = require('js-yaml');

const app = new Koa();
const router = new Router();

async function show(ctx) {
  ctx.body = yaml.load(ctx.request.body);
}

router.get('/config', show);
app.use(router.routes());

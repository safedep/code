import { createApp, defineAsyncComponent } from 'vue';

const pages = import.meta.glob('./pages/*.vue');
const app = createApp({});
for (const [path, load] of Object.entries(pages)) {
  app.component(path, defineAsyncComponent(load));
}
app.mount('#app');

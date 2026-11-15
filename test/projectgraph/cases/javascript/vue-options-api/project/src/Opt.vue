<template>
  <button @click="go">go</button>
</template>

<script>
import { odanger } from 'plug';

export default {
  methods: {
    go() {
      odanger();
    },
  },
};
</script>

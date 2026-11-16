<script setup>
import { render } from 'plug';
import { validate } from 'jsonschema';

render();
</script>

<template>
  <p>{{ validate(1) }}</p>
</template>

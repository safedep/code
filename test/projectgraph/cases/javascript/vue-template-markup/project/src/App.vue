<template>
  <button @click="danger(item)">{{ render(item) }}</button>
</template>

<script setup lang="ts">
import { danger, render, setup } from 'plug';

const item: string = setup('x');
</script>

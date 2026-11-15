<template>
  <button @click="onClick">go</button>
</template>

<script setup>
import { danger } from 'plug';

function onClick() {
  danger();
}
</script>
